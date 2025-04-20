package ui_util

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type HtmlTemplate struct {
	Name          string
	FileOverrides []string
}

type TemplateLoader struct {
	templatesBaseDir          string // included in every template
	templatesOverridesDir     string // only added if each file listed individually in the HtmlTemplate struct
	htmlTemplates             []HtmlTemplate
	// templateMap a map is required as I want to be able to override specific templates (e.g., swap out a table body)
	templateMap map[string]*template.Template
}

func NewTemplateLoader(
	templatesBaseDir string,
	templatesOverridesDir string,
	htmlTemplates []HtmlTemplate,
	localMode bool,
) (*TemplateLoader, error) {
	tl := TemplateLoader{
		templatesBaseDir:      templatesBaseDir,
		templatesOverridesDir: templatesOverridesDir,
		htmlTemplates:             htmlTemplates,
		templateMap:               make(map[string]*template.Template),
	}
	err := tl.parseTemplates()
	if err != nil {
		return nil, fmt.Errorf("error, when parseTemplates() for main(). Error: %v", err)
	}
	if localMode {
		for _, templatesDir := range []string{tl.templatesBaseDir, tl.templatesOverridesDir} {
			go func(td string) {
				watcher, err := fsnotify.NewWatcher()
				if err != nil {
					log.Fatalf("error, when creating new watcher. Error: %v", err)
				}
				defer watcher.Close()
				err = watcher.Add(td)
				if err != nil {
					log.Fatalf("error, when adding templates directory to be watched. Error: %v", err)
				}
				err2 := watchFiles(watcher)
				if err2 != nil {
					log.Fatalf("error, when watchFiles() for main(). Error: %v", err2)
				}
			}(templatesDir)
		}
	}
	return &tl, nil
}

func InitStaticFiles(mux *http.ServeMux, staticFilesPath string) {
    mux.Handle(
        "/static/",
        http.StripPrefix(
            "/static",
            http.FileServer(
                http.Dir(
                    staticFilesPath,
                ),
            ),
        ),
    )
}

func (tl *TemplateLoader) GetTemplateGroup(name string) *template.Template {
	return tl.templateMap[name]
}

type debouncer struct {
	delay   time.Duration
	timer   *time.Timer
	mu      sync.Mutex
	started bool
}

func newDebouncer(delay time.Duration) *debouncer {
	return &debouncer{
		delay: delay,
	}
}

func (d *debouncer) debounce(f func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// If a timer is already running, stop it
	if d.timer != nil {
		d.timer.Stop()
	}

	// Start a new timer that will call the function after the delay
	d.timer = time.AfterFunc(d.delay, func() {
		d.mu.Lock() // we are locking before the function call because there could be multple
		// clients connected, perhaps debugging on two different device clients (i.e., different OS or different browser)
		f()
		d.timer = nil // Allow the next function call to use a new timer
		d.mu.Unlock()
	})
}

const fileEventsBufferSize = 30

var uiFilesBeChangin chan fsnotify.Event = make(chan fsnotify.Event, fileEventsBufferSize)
var uiFilesBeChanginTimes chan int64 = make(chan int64, fileEventsBufferSize)

func (tl *TemplateLoader) HandleHotReload(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// watch for events and enrich with current time
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-uiFilesBeChangin:
				currentTime := time.Now().UnixMilli()
				if isChangedEvent(event) {
					log.Printf(
						"file change event detected. File: %s. Operation: %s. Time: %d",
						event.Name,
						event.Op.String(),
						currentTime,
					)
					log.Printf("starting channel at %d", time.Now().UnixMilli())
					uiFilesBeChanginTimes <- currentTime
					log.Printf("draining channel at %d", time.Now().UnixMilli())
				} else {
					log.Printf(
						"file non change event detected. File: %s. Operation: %s. Time: %d",
						event.Name,
						event.Op.String(),
						currentTime,
					)
				}
			}
		}
	}()

	SendSseHeaders(w)
	i := 0
	retrySleep := 250 * time.Millisecond
	// wait till all file operations have settled, this ensures the
	// files are in the desired state when we parse them
	// debounce 50 miliseconds, as it seems file events are around 6 miliseconds apart and there are 3-4 each time
	db := newDebouncer(50 * time.Millisecond)
	sb := strings.Builder{}
	for {
		select {
		case <-ctx.Done():
			return
		case eventTime := <-uiFilesBeChanginTimes:
			db.debounce(func() {
				_, err := sb.WriteString(fmt.Sprintf("name=%d@time=%d", eventTime, time.Now().UnixMilli()))
				if err != nil {
					log.Fatalf("error, when writing event to string builder. Error: %v", err)
				}

				err2 := tl.parseTemplates()
				if err2 != nil {
					err2 = fmt.Errorf("error, when parseDevTemplates() for handleHotreload(). Error: %v", err2)
					// http.Error(w, err2.Error(), http.StatusInternalServerError) // not returning the error because in golang you can't set the status code more than once in a single call.
					// And this is a long running call so it is likely to happen more than once
					log.Printf(err2.Error()) // do not crash the program, as errors are expected during development
                    return
				}
				evt := fmt.Sprintf(
					"id:%X\nretry:%d\ndata:%s\n\n",
					i, retrySleep, sb.String(),
				)
				w.Write([]byte(evt))
				log.Printf("message sent: %d", eventTime)
				w.(http.Flusher).Flush()
				i++
			})
		}
	}
}

func isDir(path string) bool {
	i, err := os.Stat(path)
	if err != nil {
		return false
	}
	return i.IsDir()
}

func isChangedEvent(ev fsnotify.Event) bool {
	return ev.Op&fsnotify.Create == fsnotify.Create ||
		ev.Op&fsnotify.Write == fsnotify.Write ||
		ev.Op&fsnotify.Remove == fsnotify.Remove
}

func watchFiles(watcher *fsnotify.Watcher) error {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				log.Println("event watcher closed")
				return nil
			}
			uiFilesBeChangin <- event
		case err, ok := <-watcher.Errors:
			if !ok {
				log.Println("error watcher closed")
				return nil
			}
			return fmt.Errorf("error, watching html files for changes. Error: %v", err)
		}
	}
}

func (tl *TemplateLoader) parseTemplates() error {
	baseTemplate, err := tl.parseBaseTemplates()
	if err != nil {
		return fmt.Errorf("error, when TemplateLoader.parseBaseTemplates()for TemplateLoader.parseTemplates(). Error: %v", err)
	}
	for _, t := range tl.htmlTemplates {
		err = tl.parseOverrideTemplate(t, baseTemplate)
		if err != nil {
			return fmt.Errorf(
				"error, when parseOverrideTemplate() for parseTemplates() when processing template: %s. Error: %v",
				t.Name,
				err,
			)
		}
	}
	return nil
}

func (tl *TemplateLoader) parseOverrideTemplate(t HtmlTemplate, baseTemplate *template.Template) error {
	bt, err := baseTemplate.Clone()
	if err != nil {
		return fmt.Errorf("error, when cloning base templates for parseBaseTemplates(). Error: %v", err)
	}
	if len(t.FileOverrides) == 0 { // a generic base template has probably been defined if this is the case
		tl.templateMap[t.Name] = bt
		return nil
	}
	theTemplate, err := bt.ParseFS(
		os.DirFS(tl.templatesOverridesDir),
		t.FileOverrides...,
	)
	if err != nil {
		return fmt.Errorf("error, when attempting to parse templates. Error: %v", err)
	}
	tl.templateMap[t.Name] = theTemplate
	return nil
}

func (tl *TemplateLoader) parseBaseTemplates() (*template.Template, error) {
	templatesPath := fmt.Sprintf("%s/*.html", tl.templatesBaseDir)
	templateFileNames, err := filepath.Glob(templatesPath)
	if err != nil {
		return nil, fmt.Errorf("error, when trying to list files for local hosting. Error: %v", err)
	}
	for i, fullPath := range templateFileNames {
		fileName := filepath.Base(fullPath) // Get only the file name
		templateFileNames[i] = fileName
	}
	baseTemplate, err := template.ParseFS(
		os.DirFS(tl.templatesBaseDir),
		templateFileNames...,
	)
	if err != nil {
		return nil, fmt.Errorf("error, when trying to read files for local hosting. Error: %v", err)
	}
	return baseTemplate, nil
}

func SendSseHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "Keep-Alive")
	w.(http.Flusher).Flush()
}
