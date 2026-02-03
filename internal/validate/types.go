package validate

// Status represents the outcome of a validation item.
type Status int

const (
	StatusSuccess Status = iota
	StatusWarning
	StatusPending
	StatusError
)

// Item represents a single validation result.
type Item struct {
	Name    string
	Status  Status
	Details string
}

// Result captures outcomes for a validation check.
type Result struct {
	Items    []Item
	Errors   []string
	Warnings []string
	Pending  []string
}

// AddItem appends an item with status and optional details.
func (r *Result) AddItem(status Status, name, details string) {
	r.Items = append(r.Items, Item{
		Name:    name,
		Status:  status,
		Details: details,
	})
}

// AddError records an error message.
func (r *Result) AddError(msg string) {
	r.Errors = append(r.Errors, msg)
}

// AddWarning records a warning message.
func (r *Result) AddWarning(msg string) {
	r.Warnings = append(r.Warnings, msg)
}

// AddPending records a pending message.
func (r *Result) AddPending(msg string) {
	r.Pending = append(r.Pending, msg)
}
