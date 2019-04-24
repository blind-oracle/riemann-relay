package main

import (
	"fmt"
)

func eventToCarbon(e *Event) ([]byte, error) {
	pfx, err := getPrefix(e)
	if err != nil {
		return []byte{}, err
	}

	return []byte(fmt.Sprintf("%s.%s.%s %f %d", pfx, e.Host, e.Service, e.MetricF, e.Time)), nil
}

func getPrefix(e *Event) (string, error) {
	for _, a := range e.Attributes {
		if a.Key == "prefix" {
			return a.Value, nil
		}
	}

	return "", fmt.Errorf("No 'prefix' found in attributes")
}
