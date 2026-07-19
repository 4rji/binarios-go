package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const apiURL = "https://api.openai.com/v1/chat/completions"

var (
	blue  = "\033[94m"
	green = "\033[92m"
	reset = "\033[0m"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type RequestBody struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Choice struct {
	Message Message `json:"message"`
}

type ResponseBody struct {
	Choices []Choice `json:"choices"`
}

func main() {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Println("OPENAI_API_KEY no está definida")
		return
	}

	reader := bufio.NewReader(os.Stdin)
	messages := []Message{
		{
			Role: "system",
			Content: "Eres un asistente experto en programación, Linux y ciberseguridad, pentesting, hackingtools, white hat hacker. " +
				"Proporcionas respuestas cortas, el usuario es medio-avanzado. No explicar lo basico a menos que se pregunte.",
		},
	}

	fmt.Println("Amigo: What do you want?.\n")

	for {
		fmt.Print(blue + ">  " + reset)
		userInput, _ := reader.ReadString('\n')
		userInput = strings.TrimSpace(userInput)

		if userInput == "Si..." {
			fmt.Println(green + "> Sí" + reset + "\n")
			messages = append(messages, Message{Role: "assistant", Content: "Sí"})
			continue
		}
		if strings.EqualFold(userInput, "Exit") {
			fmt.Println(green + "Amigo: ¡Adios!" + reset)
			break
		}

		messages = append(messages, Message{Role: "user", Content: userInput})

		body := RequestBody{
			Model:    "gpt-4o",
			Messages: messages,
		}
		jsonData, _ := json.Marshal(body)

		req, _ := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println("Error:", err)
			break
		}
		defer resp.Body.Close()

		var responseBody ResponseBody
		json.NewDecoder(resp.Body).Decode(&responseBody)

		if len(responseBody.Choices) > 0 {
			reply := responseBody.Choices[0].Message.Content
			fmt.Println(green + "<  " + reply + reset + "\n")
			messages = append(messages, Message{Role: "assistant", Content: reply})
		} else {
			fmt.Println("No se recibió respuesta.")
			break
		}
	}
}
