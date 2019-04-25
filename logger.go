package main

import log "github.com/sirupsen/logrus"

type logger struct {
	pfx string
}

func (l *logger) Debugf(msg string, args ...interface{}) {
	log.Debugf(l.pfx+": "+msg, args...)
}

func (l *logger) Infof(msg string, args ...interface{}) {
	log.Infof(l.pfx+": "+msg, args...)
}

func (l *logger) Warnf(msg string, args ...interface{}) {
	log.Warnf(l.pfx+": "+msg, args...)
}

func (l *logger) Errorf(msg string, args ...interface{}) {
	log.Errorf(l.pfx+": "+msg, args...)
}

func (l *logger) Fatalf(msg string, args ...interface{}) {
	log.Fatalf(l.pfx+": "+msg, args...)
}
