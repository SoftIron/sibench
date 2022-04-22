// SPDX-FileCopyrightText: 2022 SoftIron Limited <info@softiron.com>
// SPDX-License-Identifier: GNU General Public License v2.0 only WITH Classpath exception 2.0


package logger

import "fmt"

/* Logging levels. */
type LogLevel int
const (
    Error LogLevel = iota
    Warn
    Info
    Debug
    Trace
)


var level LogLevel = Info


func SetLevel(l LogLevel) {
    level = l
}


func IsError() bool {
    // Error logging is always enabled.
    return true
}


func IsWarn() bool {
    return level >= Warn
}


func IsInfo() bool {
    return level >= Info
}


func IsDebug() bool {
    return level >= Debug
}


func IsTrace() bool {
    return level >= Trace
}


func Errorf(format string, args ...interface{}) {
    if IsError() {
        fmt.Printf("ERROR: " + format, args...)
    }
}


func Warnf(format string, args ...interface{}) {
    if IsWarn() {
        fmt.Printf("Warning: " + format, args...)
    }
}


func Infof(format string, args ...interface{}) {
    if IsInfo() {
        fmt.Printf(format, args...)
    }
}


func Debugf(format string, args ...interface{}) {
    if IsDebug() {
        fmt.Printf(format, args...)
    }
}


func Tracef(format string, args ...interface{}) {
    if IsTrace() {
        fmt.Printf(format, args...)
    }
}


