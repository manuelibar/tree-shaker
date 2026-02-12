package shaker

// Builder is the entry point of the fluent API, returned by From().
// It forks into IncludeBuilder or ExcludeBuilder, enforcing mutual exclusivity at compile time.
type Builder struct {
	input  []byte
	prefix string
}

func (b *Builder) Prefix(prefix string) *Builder {
	b.prefix = prefix
	return b
}

func (b *Builder) Include(paths ...string) *IncludeBuilder {
	return &IncludeBuilder{
		builder: b,
		paths:   paths,
	}
}

func (b *Builder) Exclude(paths ...string) *ExcludeBuilder {
	return &ExcludeBuilder{
		builder: b,
		paths:   paths,
	}
}

type IncludeBuilder struct {
	builder *Builder
	paths   []string
}

func (p *IncludeBuilder) Include(paths ...string) *IncludeBuilder {
	p.paths = append(p.paths, paths...)
	return p
}

func (p *IncludeBuilder) Shake() ([]byte, error) {
	q := Include(p.paths...)
	if p.builder.prefix != "" {
		q = q.WithPrefix(p.builder.prefix)
	}
	return Shake(p.builder.input, q)
}

func (p *IncludeBuilder) MustShake() []byte {
	out, err := p.Shake()
	if err != nil {
		panic(err)
	}
	return out
}

type ExcludeBuilder struct {
	builder *Builder
	paths   []string
}

func (p *ExcludeBuilder) Exclude(paths ...string) *ExcludeBuilder {
	p.paths = append(p.paths, paths...)
	return p
}

func (p *ExcludeBuilder) Shake() ([]byte, error) {
	q := Exclude(p.paths...)
	if p.builder.prefix != "" {
		q = q.WithPrefix(p.builder.prefix)
	}
	return Shake(p.builder.input, q)
}

func (p *ExcludeBuilder) MustShake() []byte {
	out, err := p.Shake()
	if err != nil {
		panic(err)
	}
	return out
}
