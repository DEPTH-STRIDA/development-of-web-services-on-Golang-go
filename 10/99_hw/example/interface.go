package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// Такой подход используется в библиотеках, например, в библиотеке tgbotapi
// У нас есть разные структуры, для отправки сообщения, редактирования, удаления и т.д.
// Но все они реализуют один и тот же интерфейс, что позволяет нам использовать их в одном месте bot.send

// Requester - интерфейс для запроса к сервису
// *Интерфейсы кончаются на "er", даже если нарушается правило речи*
type Requester interface {
	Method() string
	Body() string
	Response() interface{}
}

// Client - структура клиента для отправки запросов
type Client struct{}

// Send отправляет запрос, используя структуру, реализующую интерфейс Requester
func (c *Client) Send(requester Requester) (interface{}, error) {
	client := &http.Client{}
	req, err := http.NewRequest(requester.Method(), "https://example.com", bytes.NewBufferString(requester.Body()))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	var response interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return response, nil
}

// ExampleRequester - пример реализации интерфейса Requester
type ExampleRequester struct {
	method   string
	body     string
	response struct {
		Status string `json:"status"`
	}
}

func (e *ExampleRequester) Method() string {
	return e.method
}

func (e *ExampleRequester) Body() string {
	return e.body
}

func (e *ExampleRequester) Response() interface{} {
	return &e.response
}
