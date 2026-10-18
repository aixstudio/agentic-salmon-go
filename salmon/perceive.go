package salmon

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type VisionProvider interface{ Perceive() (Perception, error) }
type FixtureProvider struct{ FixturePath, ImagePath string }

func decodeStrict(data []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("expected one JSON value")
	}
	return nil
}
func digest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func (f FixtureProvider) Perceive() (Perception, error) {
	var p Perception
	data, err := os.ReadFile(f.FixturePath)
	if err != nil {
		return p, err
	}
	if err = decodeStrict(data, &p); err != nil {
		return p, fmt.Errorf("fixture: %w", err)
	}
	image, err := os.ReadFile(f.ImagePath)
	if err != nil {
		return p, err
	}
	if p.ImageSHA256 != digest(image) {
		return p, fmt.Errorf("input image does not match reviewed fixture")
	}
	return p, nil
}

type PerceiveAgent struct{ Provider VisionProvider }

func (a PerceiveAgent) Run() (Perception, error) {
	if a.Provider == nil {
		return Perception{}, fmt.Errorf("Perceive requires a provider")
	}
	p, err := a.Provider.Perceive()
	if err != nil {
		return p, err
	}
	if strings.TrimSpace(p.Provenance) == "" || len(p.Observations) == 0 || len(p.Unknowns) == 0 {
		return p, fmt.Errorf("Perceive requires provenance, observations, and retained unknowns")
	}
	ids := map[string]bool{}
	for _, o := range p.Observations {
		if strings.TrimSpace(o.ID) == "" || ids[o.ID] || strings.TrimSpace(o.Claim) == "" || strings.TrimSpace(o.Source) == "" {
			return p, fmt.Errorf("invalid or duplicate observation: %q", o.ID)
		}
		ids[o.ID] = true
	}
	candidates := map[string]bool{}
	for _, c := range p.Candidates {
		if strings.TrimSpace(c.ID) == "" || candidates[c.ID] {
			return p, fmt.Errorf("invalid or duplicate candidate: %q", c.ID)
		}
		candidates[c.ID] = true
		if c.Source != HumanHypothesis && c.Source != ModelAssertion {
			return p, fmt.Errorf("unknown assertion source: %q", c.Source)
		}
		for _, id := range c.SupportingIDs {
			if !ids[id] {
				return p, fmt.Errorf("candidate references unknown observation: %s", id)
			}
		}
	}
	return p, nil
}
