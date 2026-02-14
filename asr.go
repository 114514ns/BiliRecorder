package main

import "strings"

type ElevenLab struct {
}

type TranscribeText struct {
	Begin float64
	End   float64
	Text  string
}
type Speak struct {
	Speaker string
	Texts   []TranscribeText
}
type Transcribe struct {
	Data []Speak
}

func parseTranscribe(obj map[string]interface{}) Transcribe {
	var result Transcribe
	speakerMap := make(map[string]int)
	punctuations := map[string]bool{
		"。": true, "？": true, "！": true, "…": true,
		"?": true, "!": true, ".": true,
	}
	const timeGapThreshold = 1.0
	var sentBuf strings.Builder
	var sentStart = -1.0
	var lastWordEnd = 0.0
	currentBufferSpeaker := ""
	addSentenceToResult := func(spkId string, text TranscribeText) {
		idx, exists := speakerMap[spkId]
		if !exists {
			newSpeak := Speak{
				Speaker: spkId,
				Texts:   []TranscribeText{},
			}
			result.Data = append(result.Data, newSpeak)
			idx = len(result.Data) - 1
			speakerMap[spkId] = idx
		}
		if text.Text != "" && text.Text != " " {
			result.Data[idx].Texts = append(result.Data[idx].Texts, text)
		}
	}
	wordsArr := getArray(obj, "words")
	for i, w := range wordsArr {
		text := getString(w, "text")
		start := getFloat64(w, "start")
		end := getFloat64(w, "end")
		speaker := getString(w, "speaker_id")
		if i == 0 {
			currentBufferSpeaker = speaker
		}
		speakerChanged := speaker != currentBufferSpeaker
		isBigGap := (start-lastWordEnd) > timeGapThreshold && sentBuf.Len() > 0
		if speakerChanged {
			if sentBuf.Len() > 0 {
				addSentenceToResult(currentBufferSpeaker, TranscribeText{
					Begin: sentStart,
					End:   lastWordEnd,
					Text:  sentBuf.String(),
				})
			}
			currentBufferSpeaker = speaker
			sentBuf.Reset()
			sentStart = start
		} else if isBigGap {
			addSentenceToResult(currentBufferSpeaker, TranscribeText{
				Begin: sentStart,
				End:   lastWordEnd,
				Text:  sentBuf.String(),
			})
			sentBuf.Reset()
			sentStart = start
		}
		if sentBuf.Len() == 0 {
			sentStart = start
		}
		sentBuf.WriteString(text)
		lastWordEnd = end

		cleanText := strings.TrimSpace(text)
		if punctuations[cleanText] {
			addSentenceToResult(currentBufferSpeaker, TranscribeText{
				Begin: sentStart,
				End:   end,
				Text:  sentBuf.String(),
			})
			sentBuf.Reset()
		}
	}
	if sentBuf.Len() > 0 {
		addSentenceToResult(currentBufferSpeaker, TranscribeText{
			Begin: sentStart,
			End:   lastWordEnd,
			Text:  sentBuf.String(),
		})
	}
	return result
}
