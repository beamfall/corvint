package gitauth

import "context"

// UseObjectView splits live currentness metadata from an independently audited
// immutable object directory. This is not an authority-admission API: only the
// protected consumer supplies its already audited view. Ordinary CEM callers
// retain their existing repository behavior.
func (r *Repository) UseObjectView(ctx context.Context, view *Repository) error {
	if view == nil || view == r || view.objectView != nil {
		return unavailable("invalid object view")
	}
	if err := r.LoadObjectFormat(ctx); err != nil {
		return err
	}
	if err := view.LoadObjectFormat(ctx); err != nil {
		return err
	}
	if r.ObjectFormat != view.ObjectFormat {
		return unavailable("object view format mismatch")
	}
	r.objectView = view
	return nil
}
