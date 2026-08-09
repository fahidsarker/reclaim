package plan

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Size sentinels for Target.Size after the reporting pass.
const (
	SizeSkipped int64 = -1 // --no-size
	SizeUnknown int64 = -2 // sizing failed
)

// Stats holds scan/size timing and counts for human output headers.
type Stats struct {
	DirsWalked   int
	Projects     int
	Depth        int
	ScanDuration time.Duration
	SizeDuration time.Duration
}

// SizeOptions controls the concurrent sizing pass.
type SizeOptions struct {
	Concurrency int
	NoSize      bool
}

// Size fills Target.Size and Target.ModTime on delete-candidate decisions.
// Failures degrade to SizeUnknown and never change verdicts.
func Size(p *Plan, opts SizeOptions) error {
	if p == nil {
		return nil
	}
	if opts.Concurrency < 1 {
		opts.Concurrency = 1
	}

	var idxs []int
	for i := range p.Decisions {
		if p.Decisions[i].Verdict == VerdictDelete {
			idxs = append(idxs, i)
		}
	}

	start := time.Now()
	defer func() {
		p.Stats.SizeDuration = time.Since(start)
	}()

	if opts.NoSize {
		for _, i := range idxs {
			p.Decisions[i].Target.Size = SizeSkipped
			if st, err := os.Lstat(p.Decisions[i].Target.Path); err == nil {
				p.Decisions[i].Target.ModTime = st.ModTime()
			}
		}
		return nil
	}

	if len(idxs) == 0 {
		return nil
	}

	workers := opts.Concurrency
	if workers > len(idxs) {
		workers = len(idxs)
	}

	jobs := make(chan int, len(idxs))
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				sizeOne(&p.Decisions[i])
			}
		}()
	}
	for _, i := range idxs {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return nil
}

func sizeOne(d *Decision) {
	st, err := os.Lstat(d.Target.Path)
	if err != nil {
		d.Target.Size = SizeUnknown
		return
	}
	d.Target.ModTime = st.ModTime()
	if st.Mode()&os.ModeSymlink != 0 {
		d.Target.Size = st.Size()
		return
	}
	if !st.IsDir() {
		d.Target.Size = st.Size()
		return
	}
	n, err := dirSize(d.Target.Path)
	if err != nil {
		d.Target.Size = SizeUnknown
		return
	}
	d.Target.Size = n
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.Type()&fs.ModeSymlink != 0 {
			info, lerr := os.Lstat(path)
			if lerr != nil {
				return nil
			}
			total += info.Size()
			return nil
		}
		if de.IsDir() {
			return nil
		}
		info, ierr := de.Info()
		if ierr != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
}
