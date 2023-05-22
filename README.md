# Heartbeat

`heartbeat` is useful when you want to terminate a context-aware long-running task that is not making any
progress. If the task duration is variable or unknown, using `context.WithTimeout` with a fixed value is not ideal.
Instead, `heartbeat` can cancel your task when a certain period passes since the last reported progress.

Examples of the use cases:

* External processes with progress lines printed to stdout (yt-dlp, ffmpeg, latex, etc.)
* Network streams or asynchronous API endpoints (e.g. chat completion endpoint in OpenAI.)

## Installation

```bash
go get -u ytils.dev/heartbeat@latest
```

## Example

> Check the [documentation](https://ytils.dev/heartbeat) for more details.

In the following example we start traceroute(8) and keep it alive as long as it prints something to stdout within
a minute:

```go
package main

func main() {
  ctx := context.Background()

  hb := heartbeat.New(ctx, time.Minute, &heartbeat.Options{
    CheckInterval: 10 * time.Second,
    CheckHook: func(timeout, idle, left time.Duration) {
      fmt.Printf("last activity was %s ago\n", idle.Truncate(time.Second))
    },
  })
  defer hb.Close() // to avoid goroutine leaks

  // The command will terminate if the context is canceled
  cmd := exec.CommandContext(hb.Ctx(), "traceroute", "google.com")
  stdout, _ := cmd.StdoutPipe()
  if err := cmd.Start(); err != nil {
    panic(err)
  }

  // Show the output and keep the context alive
  sc := bufio.NewScanner(stdout)
  for sc.Scan() {
    fmt.Println(sc.Text())
    hb.Beat()
  }

  <-hb.Ctx().Done()
  fmt.Println("the context is canceled!")
}
```

The output should look like this:

```go
 1  192.168.88.1 (192.168.88.1)  2.101 ms  1.774 ms  1.722 ms
2  192.168.1.254 (192.168.1.254)  2.223 ms  2.247 ms  2.119 ms
last activity was 9s ago
3  * * *
last activity was 4s ago
last activity was 14s ago
4  * * *
5  31.55.186.188 (31.55.186.188)  8.967 ms  7.838 ms

<truncated>

lhr25s33-in-f14.1e100.net (142.250.187.206)  8.438 ms
last activity was 4s ago
last activity was 14s ago
last activity was 24s ago
last activity was 34s ago
last activity was 44s ago
last activity was 54s ago
the context is canceled!
```
