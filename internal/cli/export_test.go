package cli

// SetMaxAllRows lowers the cap of `item list --all` and returns the function that restores it.
func SetMaxAllRows(n int) (restore func()) {
	old := maxAllRows
	maxAllRows = n
	return func() { maxAllRows = old }
}
