package shaker

// Builder is the entry point of the fluent builder, returned by Shaker.From().
// It forks into IncludeBuilder or ExcludeBuilder, enforcing mutual exclusivity at compile time.
type Builder struct {
	shaker *Shaker
	input  []byte
	prefix string
}

// Prefix sets a common root for all paths in this builder.
func (b *Builder) Prefix(prefix string) *Builder {
	b.prefix = prefix
	return b
}

// Include starts an include builder with the given paths.
func (b *Builder) Include(paths ...string) *IncludeBuilder {
	return &IncludeBuilder{
		builder: b,
		paths:   paths,
	}
}

// Exclude starts an exclude builder with the given paths.
func (b *Builder) Exclude(paths ...string) *ExcludeBuilder {
	return &ExcludeBuilder{
		builder: b,
		paths:   paths,
	}
}

// IncludeBuilder is a type-safe builder locked to include mode.
type IncludeBuilder struct {
	builder *Builder
	paths   []string
}

// Include adds more paths to the include set.
func (p *IncludeBuilder) Include(paths ...string) *IncludeBuilder {
	p.paths = append(p.paths, paths...)
	return p
}

// Shake executes the include builder.
func (p *IncludeBuilder) Shake() ([]byte, error) {
	q := Include(p.paths...)
	if p.builder.prefix != "" {
		q = q.WithPrefix(p.builder.prefix)
	}
	return p.builder.shaker.Shake(p.builder.input, q)
}

// MustShake executes the include builder, panicking on error.
func (p *IncludeBuilder) MustShake() []byte {
	out, err := p.Shake()
	if err != nil {
		panic(err)
	}
	return out
}

// ExcludeBuilder is a type-safe builder locked to exclude mode.
type ExcludeBuilder struct {
	builder *Builder
	paths   []string
}

// Exclude adds more paths to the exclude set.
func (p *ExcludeBuilder) Exclude(paths ...string) *ExcludeBuilder {
	p.paths = append(p.paths, paths...)
	return p
}

// Shake executes the exclude builder.
func (p *ExcludeBuilder) Shake() ([]byte, error) {
	q := Exclude(p.paths...)
	if p.builder.prefix != "" {
		q = q.WithPrefix(p.builder.prefix)
	}
	return p.builder.shaker.Shake(p.builder.input, q)
}

// MustShake executes the exclude builder, panicking on error.
func (p *ExcludeBuilder) MustShake() []byte {
	out, err := p.Shake()
	if err != nil {
		panic(err)
	}
	return out
}
