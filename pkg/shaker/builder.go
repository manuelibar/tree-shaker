package shaker

// The fluent API enforces mutual exclusivity at compile time:
// [Builder] forks into [IncludeBuilder] or [ExcludeBuilder], and the Go type
// system prevents mixing the two modes in the same chain.

// Builder is the entry point of the fluent API, returned by [From].
//
// Call [Builder.Include] or [Builder.Exclude] to fork into the
// corresponding mode. Once forked, the Go type system prevents mixing.
type Builder struct {
	input  []byte
	prefix string
}

// From creates a new [Builder] for the given JSON input.
//
//	out, err := shaker.From(input).Include("$.name", "$.email").Shake()
func From(input []byte) *Builder {
	return &Builder{
		input: input,
	}
}

// NewBuilder creates a new [Builder] for the given JSON input.
// It is an alias for [From].
func NewBuilder(input []byte) *Builder {
	return From(input)
}

// Prefix scopes all subsequent paths under prefix.
// Relative paths (starting with ".") are appended; absolute paths ("$...") are left as-is.
func (b *Builder) Prefix(prefix string) *Builder {
	b.prefix = prefix
	return b
}

// Include forks into include mode.
func (b *Builder) Include(paths ...string) *IncludeBuilder {
	return &IncludeBuilder{
		builder: b,
		paths:   paths,
	}
}

// Exclude forks into exclude mode.
func (b *Builder) Exclude(paths ...string) *ExcludeBuilder {
	return &ExcludeBuilder{
		builder: b,
		paths:   paths,
	}
}

// IncludeBuilder accumulates include paths and executes the shake.
// Additional paths can be added by chaining [IncludeBuilder.Include].
type IncludeBuilder struct {
	builder *Builder
	paths   []string
}

// Include adds paths to the query.
func (p *IncludeBuilder) Include(paths ...string) *IncludeBuilder {
	p.paths = append(p.paths, paths...)
	return p
}

// Shake applies the query and returns the pruned JSON.
func (p *IncludeBuilder) Shake() ([]byte, error) {
	q := Include(p.paths...)
	if p.builder.prefix != "" {
		q = q.WithPrefix(p.builder.prefix)
	}
	return Shake(p.builder.input, q)
}

// MustShake is like [IncludeBuilder.Shake] but panics on error.
func (p *IncludeBuilder) MustShake() []byte {
	out, err := p.Shake()
	if err != nil {
		panic(err)
	}
	return out
}

// ExcludeBuilder accumulates exclude paths and executes the shake.
// Additional paths can be added by chaining [ExcludeBuilder.Exclude].
type ExcludeBuilder struct {
	builder *Builder
	paths   []string
}

// Exclude adds paths to the query.
func (p *ExcludeBuilder) Exclude(paths ...string) *ExcludeBuilder {
	p.paths = append(p.paths, paths...)
	return p
}

// Shake applies the query and returns the pruned JSON.
func (p *ExcludeBuilder) Shake() ([]byte, error) {
	q := Exclude(p.paths...)
	if p.builder.prefix != "" {
		q = q.WithPrefix(p.builder.prefix)
	}
	return Shake(p.builder.input, q)
}

// MustShake is like [ExcludeBuilder.Shake] but panics on error.
func (p *ExcludeBuilder) MustShake() []byte {
	out, err := p.Shake()
	if err != nil {
		panic(err)
	}
	return out
}
