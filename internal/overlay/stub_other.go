//go:build !windows

package overlay

// nativeHost is a no-op stub on non-Windows platforms.
type nativeHost struct {
	state *State
}

func newNativeHost(state *State) *nativeHost {
	return &nativeHost{state: state}
}

func (h *nativeHost) show() error    { return nil }
func (h *nativeHost) hide() error    { return nil }
func (h *nativeHost) destroy() error { return nil }
func (h *nativeHost) redraw()        {}
