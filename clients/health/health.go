package health

import (
    "log"
)

func Monitor() error {
    // todo implement
    return nil
}

func HandleUnexpected(err error) {
    log.Printf(err.Error())

}
