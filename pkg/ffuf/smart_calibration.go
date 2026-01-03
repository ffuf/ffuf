package ffuf

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
)

// ResponseSignature - сигнатура ответа для группировки похожих ответов
type ResponseSignature struct {
	StatusCode    int64
	ContentLength int64
	ContentWords  int64
	ContentLines  int64
}

// SmartCalibrationStats - статистика для умной калибровки
type SmartCalibrationStats struct {
	sync.Mutex
	Responses       []Response
	SignatureCounts map[ResponseSignature]int
	TotalCollected  int
	CalibrationDone bool
}

// NewSmartCalibrationStats - создание новой статистики
func NewSmartCalibrationStats() *SmartCalibrationStats {
	return &SmartCalibrationStats{
		Responses:       make([]Response, 0),
		SignatureCounts: make(map[ResponseSignature]int),
		TotalCollected:  0,
		CalibrationDone: false,
	}
}

// GetSignature - получить сигнатуру ответа
func GetSignature(resp Response) ResponseSignature {
	return ResponseSignature{
		StatusCode:    resp.StatusCode,
		ContentLength: resp.ContentLength,
		ContentWords:  resp.ContentWords,
		ContentLines:  resp.ContentLines,
	}
}

// AddResponse - добавить ответ в статистику
func (s *SmartCalibrationStats) AddResponse(resp Response) {
	s.Lock()
	defer s.Unlock()

	s.Responses = append(s.Responses, resp)
	sig := GetSignature(resp)
	s.SignatureCounts[sig]++
	s.TotalCollected++
}

// ShouldCalibrate - проверить, нужна ли калибровка (собрано достаточно данных)
func (s *SmartCalibrationStats) ShouldCalibrate(sampleSize int) bool {
	s.Lock()
	defer s.Unlock()
	return s.TotalCollected >= sampleSize && !s.CalibrationDone
}

// IsCalibrationDone - проверить, завершена ли калибровка
func (s *SmartCalibrationStats) IsCalibrationDone() bool {
	s.Lock()
	defer s.Unlock()
	return s.CalibrationDone
}

// SetCalibrationDone - установить флаг завершения калибровки
func (s *SmartCalibrationStats) SetCalibrationDone() {
	s.Lock()
	defer s.Unlock()
	s.CalibrationDone = true
}

// SignatureCount - структура для сортировки сигнатур по количеству
type SignatureCount struct {
	Signature ResponseSignature
	Count     int
}

// AnalyzeAndGetFilters - анализ собранных ответов и получение фильтров
// thresholdPercent - процент от общего количества, при превышении которого добавляется фильтр
// Например, если thresholdPercent=50 и 60% ответов имеют одинаковый размер, этот размер будет отфильтрован
func (s *SmartCalibrationStats) AnalyzeAndGetFilters(thresholdPercent int) []FilterConfig {
	s.Lock()
	defer s.Unlock()

	filters := make([]FilterConfig, 0)
	if s.TotalCollected == 0 {
		return filters
	}

	threshold := float64(thresholdPercent) / 100.0

	// Собираем статистику по отдельным параметрам
	statusCounts := make(map[int64]int)
	sizeCounts := make(map[int64]int)
	wordCounts := make(map[int64]int)
	lineCounts := make(map[int64]int)

	for _, resp := range s.Responses {
		statusCounts[resp.StatusCode]++
		sizeCounts[resp.ContentLength]++
		wordCounts[resp.ContentWords]++
		lineCounts[resp.ContentLines]++
	}

	// Проверяем каждый параметр на превышение порога
	// Приоритет: size > words > lines > status (от более специфичного к менее)

	// Проверка по размеру
	for size, count := range sizeCounts {
		if float64(count)/float64(s.TotalCollected) >= threshold {
			filters = append(filters, FilterConfig{
				Type:  "size",
				Value: strconv.FormatInt(size, 10),
				Count: count,
			})
		}
	}

	// Если нашли фильтр по размеру, не ищем по другим параметрам
	if len(filters) > 0 {
		return filters
	}

	// Проверка по количеству слов
	for words, count := range wordCounts {
		if float64(count)/float64(s.TotalCollected) >= threshold {
			filters = append(filters, FilterConfig{
				Type:  "word",
				Value: strconv.FormatInt(words, 10),
				Count: count,
			})
		}
	}

	if len(filters) > 0 {
		return filters
	}

	// Проверка по количеству строк
	for lines, count := range lineCounts {
		if float64(count)/float64(s.TotalCollected) >= threshold {
			filters = append(filters, FilterConfig{
				Type:  "line",
				Value: strconv.FormatInt(lines, 10),
				Count: count,
			})
		}
	}

	return filters
}

// FilterConfig - конфигурация найденного фильтра
type FilterConfig struct {
	Type  string // "size", "word", "line", "status"
	Value string
	Count int
}

// GetDetailedStats - получить детальную статистику для вывода
func (s *SmartCalibrationStats) GetDetailedStats() string {
	s.Lock()
	defer s.Unlock()

	if s.TotalCollected == 0 {
		return "No responses collected yet"
	}

	// Сортируем сигнатуры по количеству
	sorted := make([]SignatureCount, 0, len(s.SignatureCounts))
	for sig, count := range s.SignatureCounts {
		sorted = append(sorted, SignatureCount{Signature: sig, Count: count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})

	result := fmt.Sprintf("Smart Calibration Stats (collected %d responses):\n", s.TotalCollected)
	result += "Most common response patterns:\n"

	// Показываем топ-5 сигнатур
	limit := 5
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for i := 0; i < limit; i++ {
		sc := sorted[i]
		percent := float64(sc.Count) / float64(s.TotalCollected) * 100
		result += fmt.Sprintf("  [%d] Status:%d Size:%d Words:%d Lines:%d - %.1f%% (%d responses)\n",
			i+1, sc.Signature.StatusCode, sc.Signature.ContentLength,
			sc.Signature.ContentWords, sc.Signature.ContentLines,
			percent, sc.Count)
	}

	return result
}
