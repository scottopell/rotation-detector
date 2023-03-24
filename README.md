Log Rotation Logger
----------

This program will run forever and monitor the given directory for any log
rotations that occur.

This only works on nix systems and will not work on windows.

It will log as much detail as possible for each "rotation" detected.

Default target dir is `/var/log/pods` and default frequency is once per second.

```
$ go build
$ ./rotation-detector -h
Usage of ./rotation-detector:
  -d, --directories stringArray   Directories to monitor. Will recursively look for any files, conceptually adds '**/*'. (default [/var/log/pods])
  -p, --period duration           How long to sleep between scans (seconds) (default 1s)
Scans the specified directories for files that have rotated (think logrotate)
```

### Docker
Available as a docker image at `docker.io/scottopelldd/rotation-detector`

```
$ docker run -v /tmp/logs:/var/log/pods docker.io/scottopelldd/rotation-detector:latest
```

