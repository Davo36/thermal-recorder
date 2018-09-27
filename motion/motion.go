package motion

import (
	"log"

	"github.com/TheCacophonyProject/lepton3"
)

const NO_DATA = 100
const TOO_MANY_POINTS_CHANGED = 200

var ZERO_FRAME lepton3.Frame

func NewMotionDetector(args MotionConfig) *motionDetector {

	d := new(motionDetector)
	d.flooredFrames = *NewFrameLoop(args.FrameCompareGap + 1)
	d.diffFrames = *NewFrameLoop(2)
	d.useOneDiff = args.UseOneDiffOnly
	d.framesGap = uint64(args.FrameCompareGap)
	d.deltaThresh = args.DeltaThresh
	d.countThresh = args.CountThresh
	d.tempThresh = args.TempThresh
	totalPixels := lepton3.FrameRows * lepton3.FrameCols
	d.nonzeroLimit = totalPixels * args.NonzeroMaxPercent / 100
	d.verbose = args.Verbose
	d.warmerOnly = args.WarmerOnly

	return d
}

type motionDetector struct {
	flooredFrames FrameLoop
	diffFrames    FrameLoop
	firstDiff     bool
	useOneDiff    bool
	tempThresh    uint16
	deltaThresh   uint16
	countThresh   int
	nonzeroLimit  int
	framesGap     uint64
	verbose       bool
	warmerOnly    bool
}

func (d *motionDetector) Detect(frame *lepton3.Frame, outMask *MotionState) bool {
	movement, _ := d.pixelsChanged(frame, outMask)
	return movement
}

func (d *motionDetector) pixelsChanged(frame *lepton3.Frame, outMask *MotionState) (bool, *MotionState) {
	outMask.Zero()

	processedFrame := d.flooredFrames.Current()
	d.setFloor(frame, processedFrame)

	// we will compare with the oldest saved frame.
	compareFrame := d.flooredFrames.Oldest()
	defer d.flooredFrames.Move()

	diffFrame := d.diffFrames.Current()
	if d.warmerOnly {
		warmerDiffFrames(processedFrame, compareFrame, diffFrame)
	} else {
		absDiffFrames(processedFrame, compareFrame, diffFrame)
	}
	prevDiffFrame := d.diffFrames.Move()

	if !d.firstDiff {
		d.firstDiff = true
		outMask.DetectionState = MotionState_NoData
		return false, outMask
	}

	hasMotion := false
	if d.useOneDiff {
		hasMotion = d.hasMotion(diffFrame, nil, outMask)
	} else {
		hasMotion = d.hasMotion(diffFrame, prevDiffFrame, outMask)
	}
	return hasMotion, outMask
}

func (d *motionDetector) setFloor(f, out *lepton3.Frame) *lepton3.Frame {
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			v := f[y][x]
			if v < d.tempThresh {
				out[y][x] = d.tempThresh
			} else {
				out[y][x] = v
			}
		}
	}
	return out
}

func (d *motionDetector) CountPixelsTwoCompare(f1, f2 *lepton3.Frame, outMask *MotionState) (nonZeros, deltas int) {
	var nonzeroCount int
	var deltaCount int
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			v1 := f1[y][x]
			v2 := f2[y][x]
			if (v1 > 0) || (v2 > 0) {
				nonzeroCount++
				if (v1 > d.deltaThresh) && (v2 > d.deltaThresh) {
					outMask.Mask[y][x] = true
					deltaCount++
				}
			}
		}
	}
	return nonzeroCount, deltaCount
}

func (d *motionDetector) CountPixels(f1 *lepton3.Frame, outMask *MotionState) (nonZeros, deltas int) {
	var nonzeroCount int
	var deltaCount int
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			v1 := f1[y][x]
			if v1 > 0 {
				nonzeroCount++
				if v1 > d.deltaThresh {
					outMask.Mask[y][x] = true
					deltaCount++
				}
			}
		}
	}
	return nonzeroCount, deltaCount
}

func (d *motionDetector) hasMotion(f1, f2 *lepton3.Frame, outMask *MotionState) bool {
	var nonzeroCount int
	var deltaCount int
	if d.useOneDiff {
		nonzeroCount, deltaCount = d.CountPixels(f1, outMask)
	} else {
		nonzeroCount, deltaCount = d.CountPixelsTwoCompare(f1, f2, outMask)
	}

	// Motion detection is suppressed when over nonzeroLimit motion
	// pixels are nonzero. This is to deal with sudden jumps in the
	// readings as the camera recalibrates due to rapid temperature
	// change.

	if nonzeroCount > d.nonzeroLimit {
		log.Printf("Motion detector - too many points changed, probably a recalculation")
		d.flooredFrames.SetAsOldest()
		d.firstDiff = false
		outMask.DetectionState = MotionState_TooManyPoints
		return false
	}

	return deltaCount >= d.countThresh
}

func absDiffFrames(a, b, out *lepton3.Frame) *lepton3.Frame {
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			out[y][x] = absDiff(a[y][x], b[y][x])
		}
	}
	return out
}

func warmerDiffFrames(a, b, out *lepton3.Frame) *lepton3.Frame {
	for y := 0; y < lepton3.FrameRows; y++ {
		for x := 0; x < lepton3.FrameCols; x++ {
			out[y][x] = warmerDiff(a[y][x], b[y][x])
		}
	}
	return out
}

func absDiff(a, b uint16) uint16 {
	d := int32(a) - int32(b)

	if d < 0 {
		return uint16(-d)
	}
	return uint16(d)
}

func warmerDiff(a, b uint16) uint16 {
	d := int32(a) - int32(b)

	if d < 0 {
		return 0
	}
	return uint16(d)
}
