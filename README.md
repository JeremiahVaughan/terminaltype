It's a typing game in your terminal.

To Play:
`ssh terminaltype.com`

ASCII art generated with:
`https://patorjk.com/software/taag/#p=display&h=0&f=Blocks&t=Term%0Ainal%20%0AType`

For running this game yourself and want to use port 22 make sure you remap your existing OpenSSH server:
    `sudo vim /etc/systemd/system/ssh.service.requires/ssh.socket`
    `sudo systemctl daemon-reload`
    `sudo systemctl restart ssh.socket`


Deploy:
```./deploy.sh```

Turn off deployed service:
```sudo systemctl stop terminaltype.service```


Ensure local ~/.ssh/config file contains an entry for "deploy.target"

See logs:
```journalctl -u terminaltype.service```


Config
    Upload with:
```aws s3 cp <local_file_path> s3://<bucket_name>/<object_key>```
    Download with:
```aws s3 cp s3://<bucket_name>/<object_key> <local_file_path>```

Config locations:
    Config is grabbed from s3 but make sure you emplace s3 files at:
```/root/.aws/config```
```/root/.aws/credentials```

