package rclone

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// Each CSI publish needs an independent VFS, while RC refresh must reach all
// publishes of the same remote. Exercise these contracts against the actual
// vendored dependency so a vendor refresh cannot silently remove our patches.
func TestVendoredVFSIndependentMountLifecycleAndRefresh(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	backend, err := fs.NewFs(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	options := vfscommon.Opt
	options.PollInterval = 0
	options.CacheMode = vfscommon.CacheModeOff
	first := vfs.New(ctx, backend, &options)
	second := vfs.New(ctx, backend, &options)
	defer second.Shutdown()
	if first == second {
		t.Fatal("independent CSI mounts reused a VFS")
	}
	for _, instance := range []*vfs.VFS{first, second} {
		if _, err := instance.ReadDir(""); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "marker"), []byte("still-mounted"), 0600); err != nil {
		t.Fatal(err)
	}
	refresh := rc.Calls.Get("vfs/refresh")
	if refresh == nil {
		t.Fatal("missing VFS refresh endpoint")
	}
	if _, err := refresh.Fn(ctx, rc.Params{"fs": fs.ConfigString(backend)}); err != nil {
		t.Fatal(err)
	}
	for _, instance := range []*vfs.VFS{first, second} {
		if _, err := instance.Stat("marker"); err != nil {
			t.Fatalf("refresh missed duplicate VFS: %v", err)
		}
	}
	first.Shutdown()
	handle, err := second.OpenFile("marker", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("unpublishing first mount broke second: %v", err)
	}
	defer handle.Close()
	data, err := io.ReadAll(handle)
	if err != nil || string(data) != "still-mounted" {
		t.Fatalf("second mount read failed: %q %v", data, err)
	}
}
