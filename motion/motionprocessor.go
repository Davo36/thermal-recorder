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
	"errors"
	"log"

	"github.com/TheCacophonyProject/lepton3"
	"github.com/TheCacophonyProject/window"

	"github.com/TheCacophonyProject/thermal-recorder/recorder"
)

func NewMotionProcessor(motionConf *MotionConfig,
	recorderConf *recorder.RecorderConfig,
	listener RecordingListener,
	recorder recorder.Recorder) *MotionProcessor {

	return &MotionProcessor{
		minFrames:      recorderConf.MinSecs * lepton3.FramesHz,
		maxFrames:      recorderConf.MaxSecs * lepton3.FramesHz,
		motionDetector: NewMotionDetector(*motionConf),
		frameLoop:      NewFrameLoop(recorderConf.PreviewSecs*lepton3.FramesHz + motionConf.TriggerFrames),
		isRecording:    false,
		window:         *window.New(recorderConf.WindowStart.Time, recorderConf.WindowEnd.Time),
		listener:       listener,
		conf:           recorderConf,
		triggerFrames:  motionConf.TriggerFrames,
		recorder:       recorder,
	}
}

type MotionProcessor struct {
	minFrames      int
	maxFrames      int
	framesWritten  int
	motionDetector *motionDetector
	frameLoop      *FrameLoop
	isRecording    bool
	totalFrames    int
	writeUntil     int
	lastLogFrame   int
	window         window.Window
	conf           *recorder.RecorderConfig
	listener       RecordingListener
	triggerFrames  int
	triggered      int
	recorder       recorder.Recorder
}

type RecordingListener interface {
	MotionDetected()
	RecordingStarted()
	RecordingEnded()
}

func (mp *MotionProcessor) Process(rawFrame *lepton3.RawFrame) {
	frame := mp.frameLoop.Current()
	rawFrame.ToFrame(frame)

	mp.internalProcess(frame)
}

func (mp *MotionProcessor) internalProcess(frame *lepton3.Frame) {
	mp.totalFrames++

	if mp.motionDetector.Detect(frame) {
		if mp.listener != nil {
			mp.listener.MotionDetected()
		}
		mp.triggered++

		if mp.isRecording {
			// increase the length of recording
			mp.writeUntil = min(mp.framesWritten+mp.minFrames, mp.maxFrames)
		} else if mp.triggered < mp.triggerFrames {
			// Only start recording after n (triggerFrames) consecutive frames with motion detected.
		} else if err := mp.canStartWriting(); err != nil {
			mp.occasionallyWriteError("Recording not started", err)
		} else if err := mp.startRecording(); err != nil {
			mp.occasionallyWriteError("Can't start recording file", err)
		} else {
			mp.writeUntil = mp.minFrames
		}
	} else {
		mp.triggered = 0
	}

	// If recording, write the frame.
	if mp.isRecording {
		err := mp.recorder.WriteFrame(frame)
		if err != nil {
			log.Printf("Failed to write to CPTV file %v", err)
		}
		mp.framesWritten++
	}

	mp.frameLoop.Move()

	if mp.isRecording && mp.framesWritten >= mp.writeUntil {
		err := mp.stopRecording()
		if err != nil {
			log.Printf("Failed to stop recording CPTV file %v", err)
		}
	}
}

func (mp *MotionProcessor) ProcessFrame(srcFrame *lepton3.Frame) {

	frame := mp.frameLoop.Current()
	frame.Copy(srcFrame)

	mp.internalProcess(frame)
}

func (mp *MotionProcessor) GetRecentFrame(frame *lepton3.Frame) *lepton3.Frame {
	return mp.frameLoop.CopyRecent(frame)
}

func (mp *MotionProcessor) canStartWriting() error {
	if !mp.window.Active() {
		return errors.New("motion detected but outside of recording window")
	} else {
		return mp.recorder.CheckCanRecord()
	}
}

func (mp *MotionProcessor) occasionallyWriteError(task string, err error) {
	shouldLogMotion := (mp.lastLogFrame == 0) //|| (mp.totalFrames >= mp.lastLogFrame+(10*framesHz))
	if shouldLogMotion {
		log.Printf("%s (%d): %v", task, mp.totalFrames, err)
		mp.lastLogFrame = mp.totalFrames
	}
}

func (mp *MotionProcessor) startRecording() error {

	var err error

	if err = mp.recorder.StartRecording(); err != nil {
		return err
	}

	mp.isRecording = true
	if mp.listener != nil {
		mp.listener.RecordingStarted()
	}

	err = mp.recordPreTriggerFrames()
	return err
}

func (mp *MotionProcessor) stopRecording() error {
	if mp.listener != nil {
		mp.listener.RecordingEnded()
	}

	err := mp.recorder.StopRecording()

	mp.framesWritten = 0
	mp.writeUntil = 0
	mp.isRecording = false
	mp.triggered = 0
	// if it starts recording again very quickly it won't write the same frames again
	mp.frameLoop.SetAsOldest()

	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (mp *MotionProcessor) recordPreTriggerFrames() error {
	frames := mp.frameLoop.GetHistory()
	var frame *lepton3.Frame
	ii := 0

	// it never writes the current frame as this will be written later
	for ii < len(frames)-1 {
		frame = frames[ii]
		if err := mp.recorder.WriteFrame(frame); err != nil {
			return err
		}
		ii++
	}

	return nil
}
