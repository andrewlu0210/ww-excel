# WW Excel

以 Go + Wails3 + Svelte 5 開發的 macOS 試算表 App，目標是提供接近 MS Excel 的操作體驗。

架構、開發規範與分階段計畫請見 [CLAUDE.md](CLAUDE.md)，效能量測紀錄請見 [docs/perf.md](docs/perf.md)。

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
export PATH="$HOME/go/bin:$PATH"
wails3 dev     # development
wails3 build   # production build -> bin/ww-excel
```
