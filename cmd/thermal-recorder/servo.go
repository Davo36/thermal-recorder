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
	"fmt"
	"io/ioutil"
	"math"
	"time"

	"github.com/felixge/pidctrl"
)

const pwmFreq = 50
const servoMinPw = 700
const servoMaxPw = 2200
const servoTravel = 180

type ServoController struct {
	active    bool
	pin       string  // pin the servo is connected to
	ang       float64 // current angle of the servo
	minAng    float64 // minimum angle of the servo
	maxAng    float64 // maximum angle of the servo
	targetAng float64 // angle to the target.
	startAng  float64 // angle to start at
	// Servo limits
	minPw  float64 // minimum pulse width for servo
	maxPw  float64 // maximum pulse width for servo
	travel float64 // angle travle from min to max pulse width
}

type TurretController struct {
	Active bool
	PID    []float64
	ServoX ServoController
	ServoY ServoController
}

func NewTurretController(conf TurretConfig) *TurretController {
	t := &TurretController{
		Active: conf.Active,
		PID:    conf.PID,
		ServoX: *NewServoController(conf.ServoX),
		ServoY: *NewServoController(conf.ServoY),
	}
	return t
}

// NewServoController used for controlling an individual servo
func NewServoController(conf ServoConfig) *ServoController {
	s := &ServoController{
		active:    conf.Active,
		pin:       conf.Pin,
		minAng:    conf.MinAng,
		maxAng:    conf.MaxAng,
		targetAng: 0,
		startAng:  (conf.MaxAng + conf.MinAng) / 2,
		minPw:     servoMinPw,
		maxPw:     servoMaxPw,
		travel:    servoTravel,
	}
	s.writeAng(s.startAng)
	return s
}

// Update uses the motionDetector to see where the servos should point
func (t *TurretController) Update(targetX, targetY int) {
	if !t.Active {
		return
	}
	t.ServoX.updateTargetAng((float64(targetX-80) * 56 / 160))
	t.ServoY.updateTargetAng((float64(targetY-60) * 56 / 160))
}

// updates the angle to the target as seen by the camera
func (s *ServoController) updateTargetAng(newAng float64) {
	s.targetAng = newAng
}

func (s *ServoController) Start(pidvals []float64) {
	pid := pidctrl.NewPIDController(pidvals[0], pidvals[1], pidvals[2])
	pid.SetOutputLimits(-50, 50)
	pid.Set(0)
	for {
		d := pid.Update(s.targetAng)
		s.ang += d
		s.targetAng += d
		s.writeAng(s.ang)
		time.Sleep(time.Millisecond * 20)
	}
}

// Calculate the PWM settings and writes it for the servos angle.
func (s *ServoController) writeAng(ang float64) {
	if !s.active {
		return
	}
	s.ang = ang
	s.ang = math.Max(s.ang, s.minAng)
	s.ang = math.Min(s.ang, s.maxAng)
	pw := s.minPw + s.ang*(s.maxPw-s.minPw)/s.travel
	dc := pw * pwmFreq / 1000000
	piBlaster := []byte(fmt.Sprintf("%s=%f\n", s.pin, dc))
	ioutil.WriteFile("/dev/pi-blaster", piBlaster, 0644)
}

func (t *TurretController) Start() {
	t.TestXYServos()
	go t.ServoX.Start(t.PID)
	go t.ServoY.Start(t.PID)
}

// TestXYServos Will move the servos along the circumference of it's viewing angles.
// Useful for testing mechanical collisions.
func (t *TurretController) TestXYServos() {
	t.ServoX.writeAng(t.ServoX.minAng)
	t.ServoY.writeAng(t.ServoY.minAng)
	time.Sleep(time.Second)

	t.ServoX.writeAng(t.ServoX.minAng)
	t.ServoY.writeAng(t.ServoY.maxAng)
	time.Sleep(time.Second)

	t.ServoX.writeAng(t.ServoX.maxAng)
	t.ServoY.writeAng(t.ServoY.maxAng)
	time.Sleep(time.Second)

	t.ServoX.writeAng(t.ServoX.maxAng)
	t.ServoY.writeAng(t.ServoY.minAng)
	time.Sleep(time.Second)

	t.ServoX.writeAng(t.ServoX.minAng)
	t.ServoY.writeAng(t.ServoY.minAng)
	time.Sleep(time.Second)

	t.ServoX.writeAng(t.ServoX.startAng)
	t.ServoY.writeAng(t.ServoY.startAng)
	time.Sleep(time.Second)
}
