package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"swoop/core"
	"swoop/core/chat"
	"swoop/core/netif"
	"swoop/core/protocol"
	"swoop/core/staging"
	"swoop/core/transfer"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails-bound application struct. It is a thin adapter over the
// platform-agnostic core engine.
type App struct {
	ctx    context.Context
	engine *core.Engine
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup boots the core engine, forwards engine events to the frontend, and
// wires drag-and-drop of files into the window.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	engine, err := core.New(core.Config{
		DataDir: appDataDir(),
		// DownloadsDir left empty: the engine resolves the OS-native
		// Downloads folder via core/paths.
	})
	if err != nil {
		wruntime.LogErrorf(ctx, "engine init: %v", err)
		return
	}
	a.engine = engine

	engine.OnPeersChanged(func(peers []protocol.DeviceInfo) {
		wruntime.EventsEmit(ctx, "peers:changed", peers)
	})
	engine.OnTransferOffer(func(o transfer.Offer) {
		wruntime.EventsEmit(ctx, "transfer:offer", o)
	})
	engine.OnTransferProgress(func(p transfer.Progress) {
		wruntime.EventsEmit(ctx, "transfer:progress", p)
	})
	engine.OnTransferState(func(s transfer.State) {
		wruntime.EventsEmit(ctx, "transfer:state", s)
	})
	engine.OnChatMessage(func(m chat.Message) {
		wruntime.EventsEmit(ctx, "chat:message", m)
	})
	engine.OnChatRead(func(peerID string, upToTs int64) {
		wruntime.EventsEmit(ctx, "chat:read", peerID, upToTs)
	})

	wruntime.OnFileDrop(ctx, func(_, _ int, paths []string) {
		wruntime.EventsEmit(ctx, "files:dropped", paths)
	})
	// Networking is started from the UI once the user picks a network interface
	// (see StartEngine).
}

// beforeClose runs while the window is still up. It cancels transfers and stops
// networking before WebView2/WebKit teardown to avoid shutdown hangs.
func (a *App) beforeClose(_ context.Context) (prevent bool) {
	if a.engine != nil {
		a.engine.Close()
	}
	return false
}

// shutdown is the Wails OnShutdown hook; idempotent after beforeClose.
func (a *App) shutdown(_ context.Context) {
	if a.engine != nil {
		a.engine.Close()
	}
}

// Interfaces lists the selectable network interfaces for the startup picker.
func (a *App) Interfaces() []netif.NetInterface {
	if a.engine == nil {
		return netif.List()
	}
	return a.engine.Interfaces()
}

// StartEngine brings up networking on the chosen interface ("" = auto).
func (a *App) StartEngine(ifaceName string) error {
	if a.engine == nil {
		return nil
	}
	return a.engine.Start(a.ctx, ifaceName)
}

// SelfInfo returns this device's advertised identity.
func (a *App) SelfInfo() protocol.DeviceInfo {
	if a.engine == nil {
		return protocol.DeviceInfo{}
	}
	return a.engine.Self()
}

// Peers returns the devices currently discovered on the LAN.
func (a *App) Peers() []protocol.DeviceInfo {
	if a.engine == nil {
		return nil
	}
	return a.engine.Peers()
}

// ScanPaths scans files and directories into a staging tree for the sender UI.
func (a *App) ScanPaths(paths []string) ([]staging.Entry, error) {
	return staging.Scan(paths)
}

// OpenFilePicker opens a native multi-select file dialog and returns the
// chosen paths.
func (a *App) OpenFilePicker() []string {
	paths, err := wruntime.OpenMultipleFilesDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Выберите файлы для отправки",
	})
	if err != nil {
		return nil
	}
	return paths
}

// OpenFolderPicker opens a native directory picker and returns the chosen path.
func (a *App) OpenFolderPicker() string {
	path, err := wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Выберите папку для отправки",
	})
	if err != nil {
		return ""
	}
	return path
}

// SendTo starts an outgoing transfer to the peer with the given id.
func (a *App) SendTo(deviceID string, items []protocol.SendItem) error {
	if a.engine == nil {
		return nil
	}
	return a.engine.SendTo(deviceID, items)
}

// CancelOutgoing cancels the current outgoing transfer.
func (a *App) CancelOutgoing() {
	if a.engine != nil {
		a.engine.CancelOutgoing()
	}
}

// CancelIncoming aborts the active incoming transfer.
func (a *App) CancelIncoming() {
	if a.engine != nil {
		a.engine.CancelIncoming()
	}
}

// LogPath returns the primary engine log file path (for support / debugging).
func (a *App) LogPath() string {
	if a.engine == nil {
		return ""
	}
	return a.engine.LogPath()
}

// DownloadsPath returns the folder where received files are saved.
func (a *App) DownloadsPath() string {
	if a.engine == nil {
		return ""
	}
	return a.engine.DownloadsDir()
}

// RevealDownloads opens the OS file manager at the downloads folder.
func (a *App) RevealDownloads() error {
	path := a.DownloadsPath()
	if path == "" {
		return fmt.Errorf("downloads folder is not available")
	}
	return revealPathInShell(path)
}

// RespondIncoming accepts (true) or declines (false) the pending incoming offer.
func (a *App) RespondIncoming(accept bool) {
	if a.engine != nil {
		a.engine.RespondIncoming(accept)
	}
}

// SendMessage sends a chat text message to the given peer.
func (a *App) SendMessage(deviceID string, text string) error {
	if a.engine == nil {
		return nil
	}
	return a.engine.SendMessage(deviceID, text)
}

// ChatHistory returns recent chat messages exchanged with the given peer.
func (a *App) ChatHistory(deviceID string) []chat.Message {
	if a.engine == nil {
		return nil
	}
	return a.engine.ChatHistory(deviceID, 200)
}

// MarkRead acknowledges to the peer that we have read their messages.
func (a *App) MarkRead(deviceID string) {
	if a.engine != nil {
		a.engine.MarkRead(deviceID)
	}
}

func appDataDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base, _ = os.Getwd()
	}
	return filepath.Join(base, "Swoop")
}
