package common

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// 요청 바디에서 받을 구조체 정의
type NumberRequest struct {
	Number int `json:"number" example:"100"`
}

// RestTestStreamResponse godoc
// @ID PostTestStreamResponse
// @Summary Stream response of a number decrement
// @Description Receives a number and streams the decrementing number every second until zero
// @Tags [Test] Stream Response
// @Accept  json
// @Produce  json-stream
// @Param number body NumberRequest true "Number input"
// @Success 200 {object} map[string]int "currentNumber"
// @Failure 400 {object} map[string]string "Invalid input"
// @Failure 500 {object} map[string]string "Stream failed"
// @Router /testStreamResponse [post]
func RestTestStreamResponse(c echo.Context) error {
	var req NumberRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "Invalid input"})
	}

	// Content-Type을 json-stream으로 설정
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c.Response().WriteHeader(http.StatusOK)
	enc := json.NewEncoder(c.Response())

	// TestStreamResponse 호출하여 숫자 스트림 처리 시작
	if err := TestStreamResponse(req.Number, enc, c); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "Stream failed"})
	}

	return nil
}

// TestStreamResponse 함수는 숫자를 1초마다 -1 하여 스트림으로 보내며, 0이 되면 종료합니다.
func TestStreamResponse(number int, enc *json.Encoder, c echo.Context) error {
	for number > 0 {
		// 숫자를 스트림으로 전송
		res := map[string]int{"currentNumber": number}
		if err := enc.Encode(res); err != nil {
			return err
		}
		// 출력 후 즉시 클라이언트로 전송
		c.Response().Flush()
		log.Info().Msgf("currentNumber: %d", number)

		// 1초 대기
		time.Sleep(1 * time.Second)

		// 숫자 감소
		number--
	}

	// 마지막으로 숫자가 0일 때 전송하고 종료
	res := map[string]int{"currentNumber": 0}
	if err := enc.Encode(res); err != nil {
		return err
	}
	c.Response().Flush()

	return nil
}
