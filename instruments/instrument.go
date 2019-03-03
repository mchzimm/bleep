package instruments

import (
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/bspaans/bs8bs/filters"
	"github.com/bspaans/bs8bs/generators"
	"github.com/bspaans/bs8bs/generators/derived"
	"gopkg.in/yaml.v2"
)

type DelayOptionsDef struct {
	Time   float64 `json:"time",yaml:"time"`
	Factor float64 `json:"factor",yaml:"factor"`
}

type OverdriveOptionsDef struct {
	Factor float64 `json:"factor",yaml:"factor"`
}

type FilterOptionsDef struct {
	Delay     *DelayOptionsDef     `json:"delay",yaml:"delay"`
	Overdrive *OverdriveOptionsDef `json:"overdrive",yaml:"overdrive"`
}

func (f *FilterOptionsDef) Filter() filters.Filter {
	if f.Delay != nil {
		return filters.NewDelayFilter(f.Delay.Time, f.Delay.Factor)
	} else if f.Overdrive != nil {
		return filters.NewOverdriveFilter(f.Overdrive.Factor)
	}
	panic("unknown filter")
	return nil
}

func (f *FilterOptionsDef) Validate() error {
	if f.Delay != nil {
		return nil
	} else if f.Overdrive != nil {
		return nil
	} else {
		return errors.New("Unknown filter")
	}
}

type FilterDef struct {
	Filter FilterOptionsDef `json:",inline" yaml:",inline"`
	On     GeneratorDef     `json:",inline" yaml:",inline"`
}

func (f *FilterDef) Generator() generators.Generator {
	return derived.NewFilteredGenerator(f.On.Generator(), f.Filter.Filter())
}

func (f *FilterDef) Validate() error {
	if f.Filter.Validate() != nil {
		return f.Filter.Validate()
	}
	return f.On.Validate()
}

type TransposeDef struct {
	Semitones    float64      `json:"semitones" yaml:"semitones"`
	Gain         float64      `json:"gain" yaml:"gain"`
	GeneratorDef GeneratorDef `json:",inline" yaml:",inline"`
}

func (t *TransposeDef) Generator() generators.Generator {
	return derived.NewTransposingGenerator(
		t.GeneratorDef.Generator(),
		t.Semitones,
		t.Gain,
	)
}

func (t *TransposeDef) Validate() error {
	return t.GeneratorDef.Validate()
}

type GeneratorOptionsDef struct {
	Attack  *float64 `json:"attack" yaml:"attack"`
	Decay   *float64 `json:"decay" yaml:"decay"`
	Sustain *float64 `json:"sustain" yaml:"sustain"`
	Release *float64 `json:"release" yaml:"release"`
}

func (g *GeneratorOptionsDef) Generator(gen generators.Generator) generators.Generator {
	if g.Attack != nil || g.Decay != nil || g.Sustain != nil || g.Release != nil {
		attack, decay, sustain, release := 0.1, 1.0, 0.5, 0.25
		if g.Attack != nil {
			attack = *g.Attack
		}
		if g.Decay != nil {
			decay = *g.Decay
		}
		if g.Sustain != nil {
			sustain = *g.Sustain
		}
		if g.Release != nil {
			release = *g.Release
		}
		gen = derived.NewEnvelopeGenerator(gen, attack, decay, sustain, release)
	}
	return gen
}

func (g *GeneratorOptionsDef) Validate() error {
	return nil
}

type GeneratorDef struct {
	Filter    *FilterDef           `json:"filter" yaml:"filter"`
	Transpose *TransposeDef        `json:"transpose" yaml:"transpose"`
	Sine      *GeneratorOptionsDef `json:"sine" yaml:"sine"`
	Square    *GeneratorOptionsDef `json:"square" yaml:"square"`
	Sawtooth  *GeneratorOptionsDef `json:"sawtooth" yaml:"sawtooth"`
	Triangle  *GeneratorOptionsDef `json:"triangle" yaml:"triangle"`
	Combined  []*GeneratorDef      `json:"combined" yaml:"combined"`
}

func (d *GeneratorDef) Generator() generators.Generator {
	var g generators.Generator
	if d.Sine != nil {
		g = d.Sine.Generator(generators.NewSineWaveOscillator())
	} else if d.Square != nil {
		g = d.Square.Generator(generators.NewSquareWaveOscillator())
	} else if d.Sawtooth != nil {
		g = d.Sawtooth.Generator(generators.NewSawtoothWaveOscillator())
	} else if d.Triangle != nil {
		g = d.Triangle.Generator(generators.NewTriangleWaveOscillator())
	} else if d.Filter != nil {
		g = d.Filter.Generator()
	} else if d.Transpose != nil {
		g = d.Transpose.Generator()
	} else if len(d.Combined) > 0 {
		gs := []generators.Generator{}
		for _, gen := range d.Combined {
			gs = append(gs, gen.Generator())
		}
		g = derived.NewCombinedGenerators(gs...)
	} else {
		panic("unknown generator")
	}
	return g
}

func (d *GeneratorDef) Validate() error {
	if d.Sine != nil {
		return d.Sine.Validate()
	} else if d.Square != nil {
		return d.Square.Validate()
	} else if d.Sawtooth != nil {
		return d.Sawtooth.Validate()
	} else if d.Triangle != nil {
		return d.Triangle.Validate()
	} else if d.Filter != nil {
		return d.Filter.Validate()
	} else if d.Transpose != nil {
		return d.Transpose.Validate()
	} else if len(d.Combined) > 0 {
		gs := []string{}
		for _, gen := range d.Combined {
			if err := gen.Validate(); err != nil {
				gs = append(gs, err.Error())
			}
		}
		if len(gs) == 0 {
			return nil
		}
		return errors.New(strings.Join(gs, "\n"))
	} else {
		return errors.New("Missing generator")
	}
}

type InstrumentDef struct {
	Index        int          `json:"index" yaml:"index"`
	Name         string       `json:"name" yaml:"name"`
	GeneratorDef GeneratorDef `json:",inline" yaml:",inline"`
}

func (i *InstrumentDef) Generator() generators.Generator {
	return i.GeneratorDef.Generator()
}

func (i *InstrumentDef) Validate() error {
	if err := i.GeneratorDef.Validate(); err != nil {
		return fmt.Errorf("Error in instrument [%d] %s: %s", i.Index, i.Name, err.Error())
	}
	return nil
}

type BankDef struct {
	Instruments []*InstrumentDef `json:"bank" yaml:"bank"`
}

func NewBankFromYamlFile(file string) (*BankDef, error) {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}
	result := BankDef{}
	if err := yaml.Unmarshal(contents, &result); err != nil {
		return nil, err
	}
	if len(result.Instruments) == 0 {
		return nil, fmt.Errorf("No instruments in bank def %s", file)
	}
	return &result, nil
}

func (b *BankDef) Validate() error {
	str := []string{}
	for _, instr := range b.Instruments {
		if err := instr.Validate(); err != nil {
			str = append(str, err.Error())
		}
	}
	if len(str) == 0 {
		return nil
	}
	return errors.New(strings.Join(str, "\n"))
}

func (b *BankDef) Activate() {
	for _, instr := range b.Instruments {
		fmt.Printf("Loading [%d] %s\n", instr.Index, instr.Name)
		Bank[instr.Index] = instr.Generator
	}
}