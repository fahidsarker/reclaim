package reclaim

import "github.com/fahid/reclaim/internal/detect"

// Detector identifies a project framework in a directory.
type Detector = detect.Detector

// Register adds a detector to the global registry. Safe to call from init().
func Register(d Detector) {
	detect.Register(d)
}
