package edge_tts

import (
	"fmt"
	"hawker-backend/services"
	"testing"
)

func TestGenerateAudio(t *testing.T) {
	path, err := services.GenerateAudio("瞧一瞧，看一看！...... 正宗好苹果，", "苹果促销", services.VoiceBoy, "+1%")
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("path: %s\n", path)
}
