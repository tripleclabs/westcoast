package actor

import (
	"fmt"
	"strings"
)

type topicPattern struct {
	raw      string
	segments []string
}

func parseTopicPattern(raw string) (topicPattern, error) {
	if strings.TrimSpace(raw) == "" {
		return topicPattern{}, ErrPubSubInvalidPattern
	}
	segments := strings.Split(raw, ".")
	for i, seg := range segments {
		if seg == "" {
			return topicPattern{}, ErrPubSubInvalidPattern
		}
		if strings.Contains(seg, "+") && seg != "+" {
			return topicPattern{}, ErrPubSubInvalidPattern
		}
		if strings.Contains(seg, "#") && seg != "#" {
			return topicPattern{}, ErrPubSubInvalidPattern
		}
		if seg == "#" && i != len(segments)-1 {
			return topicPattern{}, ErrPubSubInvalidPattern
		}
	}
	return topicPattern{raw: raw, segments: segments}, nil
}

func parsePublishTopic(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, ErrPubSubInvalidTopic
	}
	segments := strings.Split(raw, ".")
	for _, seg := range segments {
		if seg == "" || seg == "+" || seg == "#" {
			return nil, ErrPubSubInvalidTopic
		}
		if strings.Contains(seg, "+") || strings.Contains(seg, "#") {
			return nil, fmt.Errorf("%w: %s", ErrPubSubInvalidTopic, raw)
		}
	}
	return segments, nil
}
