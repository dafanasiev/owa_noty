package main

import (
	"github.com/kardianos/osext"
	"log"
	"os"
	"os/user"
	"path/filepath"
)

type resolvePathLocationFlag int

const (
	homeConfigDir resolvePathLocationFlag = 1
	selfDir       resolvePathLocationFlag = 2
)

func resolvePathOrDie(fileName string, locationsToSearch resolvePathLocationFlag) string {
	var rv string

	if locationsToSearch&homeConfigDir == homeConfigDir {
		currentUser, _ := user.Current()
		rv = filepath.Join(currentUser.HomeDir, ".config/owa_noty/", fileName)
		if _, err := os.Stat(rv); !os.IsNotExist(err) {
			return rv
		}
	}

	if locationsToSearch&selfDir == selfDir {
		selfDir, _ := os.Getwd()
		if _, err := os.Stat(rv); os.IsNotExist(err) {
			rv = filepath.Join(selfDir, fileName)
			if _, err := os.Stat(rv); os.IsNotExist(err) {
				selfDir, _ = osext.ExecutableFolder()
				rv = filepath.Join(selfDir, fileName)
			}
		}
	}

	if _, err := os.Stat(rv); os.IsNotExist(err) {
		log.Panicf("Cant find %s file", fileName)
	}

	return rv
}
