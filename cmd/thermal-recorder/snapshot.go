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

package main

import (
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path"
	"sync"

	"github.com/TheCacophonyProject/lepton3"
)

var (
	previousSnapshotID = 0
	mu                 sync.Mutex
)

func newSnapshot(dir string) error {
	mu.Lock()
	defer mu.Unlock()

	if processor == nil {
		return errors.New("Reading from camera has not started yet.")
	}
	f := processor.GetRecentFrame(new(lepton3.Frame))
	if f == nil {
		return errors.New("no frames yet")
	}
	g16 := image.NewGray16(image.Rect(0, 0, lepton3.FrameCols, lepton3.FrameRows))
	// Max and min are needed for normalization of the frame
	var valMax uint16
	var valMin uint16 = math.MaxUint16
	var id int
	for _, row := range f.Pix {
		for _, val := range row {
			id += int(val)
			valMax = maxUint16(valMax, val)
			valMin = minUint16(valMin, val)
		}
	}

	// Check if frame had already been processed
	if id == previousSnapshotID {
		return nil
	}
	previousSnapshotID = id

	var norm = math.MaxUint16 / (valMax - valMin)
	for y, row := range f.Pix {
		for x, val := range row {
			g16.SetGray16(x, y, color.Gray16{Y: (val - valMin) * norm})
		}
	}

	out, err := os.Create(path.Join(dir, "still.png"))
	if err != nil {
		return err
	}
	defer out.Close()
	return png.Encode(out, g16)
}

func maxUint16(a, b uint16) uint16 {
	if a > b {
		return a
	}
	return b
}

func minUint16(a, b uint16) uint16 {
	if a < b {
		return a
	}
	return b
}
