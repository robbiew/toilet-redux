package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/eiannone/keyboard"
	"github.com/muesli/reflow/wordwrap"
)

type User struct {
	Alias     string
	TimeLeft  time.Duration
	Emulation int
	NodeNum   int
	H         int
	W         int
	ModalH    int
	ModalW    int
}

const (
	ArtFileDir = "art"
	startCol   = 25
	startRow   = 12
	maxCols    = 25
	maxRows    = 5
)

var (
	DropPath            string
	timeOut             time.Duration
	localDisplay        bool
	u                   User // Global User object
	currentMessageIndex int  = -1
)

func init() {
	timeOut = 1 * time.Minute
	pathPtr := flag.String("path", "", "path to door32.sys file (optional if --local is set)")
	localDisplayPtr := flag.Bool("local", false, "use local UTF-8 display instead of CP437")
	flag.Parse()

	localDisplay = *localDisplayPtr // Set the global variable

	if localDisplay {
		// Set default values when --local is used
		u = User{
			Alias:     "SysOp",
			TimeLeft:  120 * time.Minute,
			Emulation: 1,
			NodeNum:   1,
			H:         25,
			W:         80,
			ModalH:    25,
			ModalW:    80,
		}
	} else {
		// Check for required --path argument if --local is not set
		if *pathPtr == "" {
			fmt.Fprintln(os.Stderr, "missing required -path argument")
			os.Exit(2)
		}
		DropPath = *pathPtr
	}
}

func addItem(timerManager *TimerManager) {
	reloadScreen()

	var messageBuffer strings.Builder
	PrintStringLoc(YellowHi+"Press ENTER when done."+Reset, 56, 7)
	fmt.Print("\033[?25h") // Show the cursor

	fmt.Print(BgBlue + White)
	MoveCursor(startCol, startRow)

	row, col := startRow, startCol
	inputCompleted := false
	for !inputCompleted {
		char, key, err := keyboard.GetKey()
		if err != nil {
			panic(err) // Handle error properly in production code
		}

		timerManager.ResetIdleTimer() // Resets the idle timer on key press

		switch {
		case key == keyboard.KeyEnter:
			inputCompleted = true
		case key == keyboard.KeySpace:
			// Handle space explicitly
			if len(messageBuffer.String()) < maxCols*maxRows {
				messageBuffer.WriteRune(' ')
				fmt.Print(" ")
				col++
				if col > startCol+maxCols-1 {
					col = startCol
					row++
					if row > startRow+maxRows-1 {
						inputCompleted = true
					} else {
						MoveCursor(col, row)
					}
				}
			}
		case key == keyboard.KeyBackspace || key == keyboard.KeyBackspace2:
			if col > startCol || row > startRow {
				if len(messageBuffer.String()) > 0 {
					newMessage := messageBuffer.String()[:len(messageBuffer.String())-1]
					messageBuffer.Reset()
					messageBuffer.WriteString(newMessage)
				}
				if col > startCol {
					col--
				} else if row > startRow {
					row--
					col = startCol + maxCols - 1
				}
				MoveCursor(col, row)
				fmt.Print(" ")       // Clear the character on the screen
				MoveCursor(col, row) // Move cursor back to position
			}
		default:
			if len(messageBuffer.String()) < maxCols*maxRows {
				messageBuffer.WriteRune(char)
				fmt.Printf("%c", char) // Print the character

				col++
				if col > startCol+maxCols-1 {
					col = startCol
					row++
					if row > startRow+maxRows-1 {
						inputCompleted = true // Stop if maximum rows reached
					} else {
						MoveCursor(col, row)
					}
				}
			}
		}
	}

	message := messageBuffer.String()

	fmt.Print("\033[?25l") // Hide the cursor
	fmt.Print(Reset)       // Reset colors

	// Ask to save the message
	saveMessage := askYesNo("Save this message? (Y/N)")
	if saveMessage {
		postAnon := askYesNo("Post anonymously? (Y/N) ")
		saveToFile(message, u.Alias, postAnon)
	} else {
		// Discard the message
		messageBuffer.Reset() // Clear the message buffer

		PrintStringLoc(RedHi+"Message discarded!       "+Reset, 56, 7)
		time.Sleep(1 * time.Second)

		reloadScreen()
	}
}

func askYesNo(prompt string) bool {
	for {
		PrintStringLoc(YellowHi+prompt+Reset, 56, 7)
		char, _, err := keyboard.GetKey()
		if err != nil {
			panic(err)
		}

		if char == 'y' || char == 'Y' {
			return true
		} else if char == 'n' || char == 'N' {
			return false
		}
	}
}

func reloadScreen() {
	// Clear the screen and redraw the default state
	ClearScreen()
	displayAnsiFile("art/toiletui.ans")
}

func saveToFile(message, author string, isAnonymous bool) {
	// Open the file in append mode, create it if it doesn't exist
	file, err := os.OpenFile("messages.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	processedMessage := processMessage(message)

	// Create a writer
	writer := bufio.NewWriter(file)

	// Get current time
	currentTime := time.Now().Format("01/02/06 03:04PM")

	// Format the anonymous flag
	anonymousText := "No"
	if isAnonymous {
		anonymousText = "Yes"
	}

	// Create the line to be written
	line := fmt.Sprintf("%s, %s, %s, %s\n", processedMessage, author, anonymousText, currentTime)

	// Write the line to the file
	_, err = writer.WriteString(line)
	if err != nil {
		panic(err) // handle error properly in production code
	}

	// Flush to make sure the data is written to the file
	err = writer.Flush()
	if err != nil {
		panic(err)
	}

	reloadScreen()
	displayMenu()
	loadMessage()
}

// stripAnsiEscapeCodes removes ANSI escape codes from a string
func stripAnsiEscapeCodes(str string) string {
	re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	return re.ReplaceAllString(str, "")
}

// removeNullChars removes null characters from a string
func removeNullChars(str string) string {
	return strings.ReplaceAll(str, "\x00", " ")
}

// escapeCommas escapes commas in a string
func escapeCommas(str string) string {
	return strings.ReplaceAll(str, ",", "\\,")
}

func processMessage(message string) string {
	message = stripAnsiEscapeCodes(message)
	message = removeNullChars(message)
	return escapeCommas(message)
}

func readLastMessageFromFile(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var lastLine string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lastLine = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	return extractMessage(lastLine), nil
}

func extractMessage(line string) string {
	var messageBuilder strings.Builder
	escapeNext := false

	for _, char := range line {
		if escapeNext {
			messageBuilder.WriteRune(char)
			escapeNext = false
			continue
		}

		if char == '\\' {
			escapeNext = true
			continue
		}

		if char == ',' {
			break
		}

		messageBuilder.WriteRune(char)
	}

	return messageBuilder.String()
}

func formatMessage(message string, width, height int) []string {
	wrapped := wordwrap.String(message, width)
	lines := strings.Split(wrapped, "\n")

	// Center each line horizontally
	for i, line := range lines {
		lines[i] = centerText(line, width)
	}

	// Add padding for vertical centering
	if len(lines) < height {
		topPadding := (height - len(lines)) / 2
		bottomPadding := height - len(lines) - topPadding
		for i := 0; i < topPadding; i++ {
			lines = append([]string{" "}, lines...)
		}
		for i := 0; i < bottomPadding; i++ {
			lines = append(lines, " ")
		}
	}

	return lines
}

func centerText(text string, width int) string {
	if len(text) >= width {
		return text[:width] // Truncate if text is too long
	}
	leftPadding := (width - len(text)) / 2
	rightPadding := width - len(text) - leftPadding
	return strings.Repeat(" ", leftPadding) + text + strings.Repeat(" ", rightPadding)
}

func loadMessage() {
	message, err := readLastMessageFromFile("messages.txt")
	if err != nil {
		panic(err)
	}

	formattedLines := formatMessage(message, maxCols, maxRows)

	for i, line := range formattedLines {
		// Display each line in the specified area
		PrintStringLoc(BgBlue+YellowHi+line+Reset, startCol, startRow+i)
	}

}

func displayMenu() {
	PrintStringLoc(Cyan+"["+CyanHi+"A"+Reset+Cyan+"]"+CyanHi+" Add"+Reset, 2, 10)
	PrintStringLoc(Cyan+"["+CyanHi+"N"+Reset+Cyan+"]"+CyanHi+" Next"+Reset, 2, 12)
	PrintStringLoc(Cyan+"["+CyanHi+"P"+Reset+Cyan+"]"+CyanHi+" Previous"+Reset, 2, 13)
	PrintStringLoc(Cyan+"["+CyanHi+"F"+Reset+Cyan+"]"+CyanHi+" First"+Reset, 2, 15)
	PrintStringLoc(Cyan+"["+CyanHi+"L"+Reset+Cyan+"]"+CyanHi+" Last"+Reset, 2, 16)
	PrintStringLoc(Cyan+"["+CyanHi+"Q"+Reset+Cyan+"]"+CyanHi+" Quit"+Reset, 2, 18)
}

func loadNextMessage() {
	// Logic to load and display the next message
	// Increment currentMessageIndex and load the message
}

func loadPreviousMessage() {
	// Logic to load and display the previous message
	// Decrement currentMessageIndex and load the message
}

func loadFirstMessage() {
	// Logic to load and display the first message
	// Set currentMessageIndex to 0 and load the message
}

func loadLastMessage() {
	// Logic to load and display the last message
	// Set currentMessageIndex to the last index and load the message
}

func main() {
	// Get door32.sys as user object
	u = Initialize(DropPath)

	// Exit if no ANSI capabilities (sorry!)
	if u.Emulation != 1 {
		fmt.Println("Sorry, ANSI is required to use this...")
		time.Sleep(time.Duration(2) * time.Second)
		os.Exit(0)
	}

	if err := keyboard.Open(); err != nil {
		fmt.Println(err)
	}
	defer func() {
		_ = keyboard.Close()
	}()

	// start idle and max timers
	timerManager := NewTimerManager(timeOut, u.TimeLeft)
	timerManager.StartIdleTimer()
	timerManager.StartMaxTimer()

	CursorHide()
	ClearScreen()
	displayAnsiFile("art/toiletui.ans")
	displayMenu()
	loadMessage()

	for {
		char, key, err := keyboard.GetKey()
		if err != nil {
			panic(err)
		}

		timerManager.ResetIdleTimer() // Resets the idle timer on key press

		if string(char) == ("a") || string(char) == ("A") {
			addItem(timerManager)
		} else if string(char) == ("n") || string(char) == ("N") {
			loadNextMessage()
		} else if string(char) == ("p") || string(char) == ("P") {
			loadPreviousMessage()
		} else if string(char) == ("f") || string(char) == ("F") {
			loadFirstMessage()
		} else if string(char) == ("l") || string(char) == ("L") {
			loadLastMessage()
		} else if string(char) == "q" || string(char) == "Q" || key == keyboard.KeyEsc {
			defer timerManager.StopIdleTimer()
			defer timerManager.StopMaxTimer()
			MoveCursor(1, u.H-1)
			CenterText("Goodbye!", 75)
			time.Sleep(time.Duration(1) * time.Second)
			CursorShow()
			return
		}
	}
}
