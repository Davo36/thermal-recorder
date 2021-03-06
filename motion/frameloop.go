// thermal-recorder - record thermal video footage of warm moving objects
//  Copyright (C) 2018, The Cacophony Project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package motion

import (
	"sync"

	"github.com/TheCacophonyProject/lepton3"
)

func NewFrameLoop(size int) *FrameLoop {
	frames := make([]*lepton3.Frame, size)
	for i := range frames {
		frames[i] = new(lepton3.Frame)
	}

	return &FrameLoop{
		size:          size,
		currentIndex:  0,
		frames:        frames,
		orderedFrames: make([]*lepton3.Frame, size),
		bufferFull:    false,
	}
}

const NO_OLDEST_SET = -1

// FrameLoop stores the last n frames in a loop that will be overwritten when full.
// The latest written frame can be anywhere in the list of frames.  Beware: all frames
// returned by FrameLoop will at some point be over-written.
type FrameLoop struct {
	size          int
	currentIndex  int
	frames        []*lepton3.Frame
	orderedFrames []*lepton3.Frame
	bufferFull    bool
	oldest        int
	mu            sync.Mutex
}

func (fl *FrameLoop) nextIndexAfter(index int) int {
	return (index + 1) % fl.size
}

// Move, moves the current frame one forwards and return the new frame.
// Note: data on all returned frame objects will eventually get overwritten
func (fl *FrameLoop) Move() *lepton3.Frame {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	fl.currentIndex = fl.nextIndexAfter(fl.currentIndex)

	if fl.currentIndex == 0 {
		fl.bufferFull = true
	}

	if fl.currentIndex == fl.oldest {
		fl.oldest = NO_OLDEST_SET
	}

	return fl.Current()
}

// Current returns the current frame.
// Note: data on all returned frame objects will eventually get overwritten
func (fl *FrameLoop) Current() *lepton3.Frame {
	return fl.frames[fl.currentIndex]
}

// CopyRecent returns a copy of the previous frame.
func (fl *FrameLoop) CopyRecent(f *lepton3.Frame) *lepton3.Frame {
	fl.mu.Lock()
	defer fl.mu.Unlock()

	previousIndex := (fl.currentIndex - 1 + fl.size) % fl.size
	f.Copy(fl.frames[previousIndex])
	return f
}

// GetHistory returns all the frames recorded in an slice from oldest to newest.
// Note: The returned slice will be rewritten next time GetHistory is called.
// Note: GetHistory always returns one frame even if none have been stored in the loop
func (fl *FrameLoop) GetHistory() []*lepton3.Frame {
	fullHistory := fl.getFullHistory()

	if fl.oldest == NO_OLDEST_SET {
		return fullHistory
	}

	historyLength := (fl.currentIndex-fl.oldest+fl.size)%fl.size + 1

	return fullHistory[len(fullHistory)-historyLength:]
}

func (fl *FrameLoop) getFullHistory() []*lepton3.Frame {
	if fl.currentIndex == fl.size-1 {
		copy(fl.orderedFrames[:], fl.frames[:])
		return fl.orderedFrames
	}

	nextIndex := fl.nextIndexAfter(fl.currentIndex)

	if !fl.bufferFull {
		copy(fl.orderedFrames, fl.frames[:nextIndex])
		return fl.orderedFrames[:nextIndex]
	}

	copy(fl.orderedFrames, fl.frames[nextIndex:])
	copy(fl.orderedFrames[fl.size-nextIndex:], fl.frames[:nextIndex])
	return fl.orderedFrames
}

// Oldest returns the oldest frame remembered.   This is either the next
// frame in the buffer (next to be overwritten), or the frame marked oldest
func (fl *FrameLoop) Oldest() *lepton3.Frame {
	if fl.oldest != NO_OLDEST_SET {
		return fl.frames[fl.oldest]
	}
	return fl.frames[fl.nextIndexAfter(fl.currentIndex)]
}

// SetAsOldest - Marks current frame as oldest.  This mean Oldest() will never return
// a frame that was written before this one.
func (fl *FrameLoop) SetAsOldest() *lepton3.Frame {
	fl.oldest = fl.currentIndex
	return fl.Current()
}
