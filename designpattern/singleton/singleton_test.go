package singleton

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetInstance(t *testing.T) {
	inst := GetInstance()
	assert.Same(t, inst, GetInstance())
	inst.Do()
}

func BenchmarkGetInstanceParallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			inst := GetInstance()
			if inst != GetInstance() {
				b.Error("failed")
			}
		}
	})
}
