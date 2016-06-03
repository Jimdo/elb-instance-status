[![Download on GoBuilder](http://badge.luzifer.io/v1/badge?title=Download%20on&text=GoBuilder)](https://gobuilder.me/github.com/Luzifer/elb-instance-status)
[![License: Apache v2.0](https://badge.luzifer.io/v1/badge?color=5d79b5&title=license&text=Apache+v2.0)](http://www.apache.org/licenses/LICENSE-2.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/Luzifer/elb-instance-status)](https://goreportcard.com/report/github.com/Luzifer/elb-instance-status)

# Luzifer / elb-instance-status

`elb-instance-status` is a small daemon you can run on any instance on an autoscaling group. It periodically executes commands using `/bin/bash` and checks for their exit status (0 = fine, everything else = not fine). The collected check results are exposed using an HTTP listener which then can be used by an ELB health check for that machine. This enables your autoscaling-group to react to custom health checks on your machine.

For example given you have a process eating all inodes on your machine and you have no chance to clean up these files you could use this daemon to terminate the instance as soon as the inode usage is too high. Maybe this is a bad example because file system cleanups should be possible all the time but you get the point: Something is wrong on one of your cattle-machines? Remove it.

The checks defined are executed every minute so you should take care not to do too expensive checks as they would stack up and could make your machine unstable. If you have checks taking longer than one minute you should do them using cron and only write a status file read by this daemon.

If the unhealthy threshold (default: 5 checks) is crossed the HTTP status will switch from 200 (OK) to 500 (Internal Server Error) which will cause the ELB to mark your machine unhealthy and the autoscaling-group will remove that machine. Of course you need to ensure there is a starting grace period to give your machine enough time to settle and get all checks green. And you also need to take care the new machines started as a replacement for the unhealthy ones are going to be healthy. Otherwise your whole cluster gets taken out of service.

## Usage

1. Install the daemon on your machine
2. Write a yaml file containing the checks you want to execute
3. Start the daemon
4. Put an ELB health check on your autoscaling-group using the daemons `/status` path as the check target

```bash
# curl -is localhost:3000/status
HTTP/1.1 200 OK
Date: Fri, 03 Jun 2016 10:56:13 GMT
Content-Length: 426
Content-Type: text/plain; charset=utf-8

[PASS] Ensure there are at least 30% free inodes on /var/lib/docker
[PASS] Ensure there are at least 30% free inodes on /
[PASS] Ensure docker can start a small container
[PASS] Ensure volume on /var/lib/docker is mounted
[PASS] Ensure there is at least 30% free disk space on /var/lib/docker
```

### Check format

The checks are defined in a quite simple yaml file:

```yaml
- name: Ensure there are at least 30% free inodes on /
  command: test $(df -i | grep "/$" | xargs | cut -d ' ' -f 5 | sed "s/%//") -lt 70

- name: Ensure volume on /var/lib/docker is mounted
  command: mount | grep -q /var/lib/docker

- name: Ensure docker can start a small container
  command: docker run --rm alpine /bin/sh -c "echo testing123" | grep -q testing123
```

They consist of three keys:

- `name` (required), A descriptive name of the check (do *not* use the same name twice!)
- `command` (required), The check itself. Needs to have exit code 0 if everything is fine and any other if somthing is wrong.  
  The checks are executed using `/bin/bash -c "<command>"`.
- `warn-only` (optional, default: false), Only put a WARN-line into the output but do not set HTTP status to 500
