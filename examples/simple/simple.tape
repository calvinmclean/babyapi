Output simple.gif

Require go

Set Shell "bash"
Set FontSize 32
Set Width 1200
Set Height 800
Set WindowBar Colorful
Set TypingSpeed 100ms

Type `go run main.go client todos post --data '{"title": "use babyapi"}'` Sleep 500ms  Enter

Sleep 2s

Type `go run main.go client todos list` Sleep 500ms  Enter

Sleep 5s
