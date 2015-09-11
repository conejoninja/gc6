// Copyright © 2015 Steve Francia <spf@spf13.com>.
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.
//

package commands

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golangchallenge/gc6/mazelib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"math"
)

type Maze struct {
	rooms      [][]mazelib.Room
	start      mazelib.Coordinate
	end        mazelib.Coordinate
	icarus     mazelib.Coordinate
	StepsTaken int
}

// Tracking the current maze being solved

// WARNING: This approach is not safe for concurrent use
// This server is only intended to have a single client at a time
// We would need a different and more complex approach if we wanted
// concurrent connections than these simple package variables
var currentMaze *Maze
var scores []int

// Defining the daedalus command.
// This will be called as 'laybrinth daedalus'
var daedalusCmd = &cobra.Command{
	Use:     "daedalus",
	Aliases: []string{"deadalus", "server"},
	Short:   "Start the laybrinth creator",
	Long: `Daedalus's job is to create a challenging Labyrinth for his opponent
  Icarus to solve.

  Daedalus runs a server which Icarus clients can connect to to solve laybrinths.`,
	Run: func(cmd *cobra.Command, args []string) {
		RunServer()
	},
}

func init() {
	rand.Seed(time.Now().UTC().UnixNano()) // need to initialize the seed
	gin.SetMode(gin.ReleaseMode)

	// Removed some commands from here
	RootCmd.AddCommand(daedalusCmd)
}

// Runs the web server
func RunServer() {
	// Adding handling so that even when ctrl+c is pressed we still print
	// out the results prior to exiting.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		printResults()
		os.Exit(1)
	}()

	// Using gin-gonic/gin to handle our routing
	r := gin.Default()
	v1 := r.Group("/")
	{
		v1.GET("/awake", GetStartingPoint)
		v1.GET("/move/:direction", MoveDirection)
		v1.GET("/done", End)
	}

	r.Run(":" + viper.GetString("port"))
}

// Ends a session and prints the results.
// Called by Icarus when he has reached
//   the number of times he wants to solve the laybrinth.
func End(c *gin.Context) {
	printResults()
	os.Exit(1)
}

// initializes a new maze and places Icarus in his awakening location
func GetStartingPoint(c *gin.Context) {
	initializeMaze()
	startRoom, err := currentMaze.Discover(currentMaze.Icarus())
	if err != nil {
		fmt.Println("Icarus is outside of the maze. This shouldn't ever happen")
		fmt.Println(err)
		os.Exit(-1)
	}
	mazelib.PrintMaze(currentMaze)

	c.JSON(http.StatusOK, mazelib.Reply{Survey: startRoom})
}

// The API response to the /move/:direction address
func MoveDirection(c *gin.Context) {
	var err error

	switch c.Param("direction") {
	case "left":
		err = currentMaze.MoveLeft()
	case "right":
		err = currentMaze.MoveRight()
	case "down":
		err = currentMaze.MoveDown()
	case "up":
		err = currentMaze.MoveUp()
	}

	var r mazelib.Reply

	if err != nil {
		r.Error = true
		r.Message = err.Error()
		c.JSON(409, r)
		return
	}

	s, e := currentMaze.LookAround()

	if e != nil {
		if e == mazelib.ErrVictory {
			scores = append(scores, currentMaze.StepsTaken)
			r.Victory = true
			r.Message = fmt.Sprintf("Victory achieved in %d steps \n", currentMaze.StepsTaken)
		} else {
			r.Error = true
			r.Message = err.Error()
		}
	}

	r.Survey = s

	c.JSON(http.StatusOK, r)
}

func initializeMaze() {
	currentMaze = createMaze()
}

// Print to the terminal the average steps to solution for the current session
func printResults() {
	fmt.Printf("Labyrinth solved %d times with an avg of %d steps\n", len(scores), mazelib.AvgScores(scores))
}

// Return a room from the maze
func (m *Maze) GetRoom(x, y int) (*mazelib.Room, error) {
	if x < 0 || y < 0 || x >= m.Width() || y >= m.Height() {
		return &mazelib.Room{}, errors.New("room outside of maze boundaries")
	}

	return &m.rooms[y][x], nil
}

func (m *Maze) Width() int  { return len(m.rooms[0]) }
func (m *Maze) Height() int { return len(m.rooms) }

// Return Icarus's current position
func (m *Maze) Icarus() (x, y int) {
	return m.icarus.X, m.icarus.Y
}

// Set the location where Icarus will awake
func (m *Maze) SetStartPoint(x, y int) error {
	r, err := m.GetRoom(x, y)

	if err != nil {
		return err
	}

	if r.Treasure {
		return errors.New("can't start in the treasure")
	}

	r.Start = true
	m.icarus = mazelib.Coordinate{x, y}
	return nil
}

// Set the location of the treasure for a given maze
func (m *Maze) SetTreasure(x, y int) error {
	r, err := m.GetRoom(x, y)

	if err != nil {
		return err
	}

	if r.Start {
		return errors.New("can't have the treasure at the start")
	}

	r.Treasure = true
	m.end = mazelib.Coordinate{x, y}
	return nil
}

// Given Icarus's current location, Discover that room
// Will return ErrVictory if Icarus is at the treasure.
func (m *Maze) LookAround() (mazelib.Survey, error) {
	if m.end.X == m.icarus.X && m.end.Y == m.icarus.Y {
		fmt.Printf("Victory achieved in %d steps \n", m.StepsTaken)
		return mazelib.Survey{}, mazelib.ErrVictory
	}

	return m.Discover(m.icarus.X, m.icarus.Y)
}

// Given two points, survey the room.
// Will return error if two points are outside of the maze
func (m *Maze) Discover(x, y int) (mazelib.Survey, error) {
	if r, err := m.GetRoom(x, y); err != nil {
		return mazelib.Survey{}, nil
	} else {
		return r.Walls, nil
	}
}

// Moves Icarus's position left one step
// Will not permit moving through walls or out of the maze
func (m *Maze) MoveLeft() error {
	s, e := m.LookAround()
	if e != nil {
		return e
	}
	if s.Left {
		return errors.New("Can't walk through walls")
	}

	x, y := m.Icarus()
	if _, err := m.GetRoom(x-1, y); err != nil {
		return err
	}

	m.icarus = mazelib.Coordinate{x - 1, y}
	m.StepsTaken++
	return nil
}

// Moves Icarus's position right one step
// Will not permit moving through walls or out of the maze
func (m *Maze) MoveRight() error {
	s, e := m.LookAround()
	if e != nil {
		return e
	}
	if s.Right {
		return errors.New("Can't walk through walls")
	}

	x, y := m.Icarus()
	if _, err := m.GetRoom(x+1, y); err != nil {
		return err
	}

	m.icarus = mazelib.Coordinate{x + 1, y}
	m.StepsTaken++
	return nil
}

// Moves Icarus's position up one step
// Will not permit moving through walls or out of the maze
func (m *Maze) MoveUp() error {
	s, e := m.LookAround()
	if e != nil {
		return e
	}
	if s.Top {
		return errors.New("Can't walk through walls")
	}

	x, y := m.Icarus()
	if _, err := m.GetRoom(x, y-1); err != nil {
		return err
	}

	m.icarus = mazelib.Coordinate{x, y - 1}
	m.StepsTaken++
	return nil
}

// Moves Icarus's position down one step
// Will not permit moving through walls or out of the maze
func (m *Maze) MoveDown() error {
	s, e := m.LookAround()
	if e != nil {
		return e
	}
	if s.Bottom {
		return errors.New("Can't walk through walls")
	}

	x, y := m.Icarus()
	if _, err := m.GetRoom(x, y+1); err != nil {
		return err
	}

	m.icarus = mazelib.Coordinate{x, y + 1}
	m.StepsTaken++
	return nil
}

// Creates a maze without any walls
// Good starting point for additive algorithms
func emptyMaze() *Maze {
	z := Maze{}
	ySize := viper.GetInt("height")
	xSize := viper.GetInt("width")

	z.rooms = make([][]mazelib.Room, ySize)
	for y := 0; y < ySize; y++ {
		z.rooms[y] = make([]mazelib.Room, xSize)
		for x := 0; x < xSize; x++ {
			z.rooms[y][x] = mazelib.Room{}
		}
	}

	return &z
}

// Creates a maze with all walls
// Good starting point for subtractive algorithms
func fullMaze() *Maze {
	z := emptyMaze()
	ySize := viper.GetInt("height")
	xSize := viper.GetInt("width")

	for y := 0; y < ySize; y++ {
		for x := 0; x < xSize; x++ {
			z.rooms[y][x].Walls = mazelib.Survey{true, true, true, true}
		}
	}

	return z
}


func backtrackerMaze() *Maze {
	z := fullMaze()
	ySize := viper.GetInt("height")
	xSize := viper.GetInt("width")
	stackSize := ySize*xSize
	stackIndex := 0
	stack := make([]mazelib.Coordinate, xSize*ySize)
	x := rand.Intn(xSize)
	y := rand.Intn(ySize)
	lastC := [4]bool{false, false, false, false}
	lastCell := 5

	stack[stackIndex] = mazelib.Coordinate{x, y}

	c := 0
	for c < stackSize{

		free := 4
		for n:=0; n<4; n++ {
			 t := (1+lastCell+n)%4

			switch (t) {
			case 0:
				if (y-1)<0 {
					lastC[0] = true
					free--
				} else {
					lastC[0] = z.rooms[y-1][x].Visited
					if lastC[0] {
						free--
					}
				}
				break
			case 1:
				if (x+1)>=xSize {
					lastC[1] = true
					free--
				} else {
					lastC[1] = z.rooms[y][x+1].Visited
					if lastC[1] {
						free--
					}
				}
				break
			case 2:
				if (y+1)>=ySize {
					lastC[2] = true
					free--
				} else {
					lastC[2] = z.rooms[y+1][x].Visited
					if lastC[2] {
						free--
					}
				}
				break
			case 3:
				if (x-1)<0 {
					lastC[3] = true
					free--
				} else {
					lastC[3] = z.rooms[y][x-1].Visited
					if lastC[3] {
						free--
					}
				}
				break
			}
		}

		if free==0 {
			lastCell = (lastCell+2)%4
			lastC[lastCell] = true
			stackIndex--
			x = stack[stackIndex].X
			y = stack[stackIndex].Y
		} else {
			t := rand.Intn(free)
			tm := 0
			for n:=0; n<4; n++ {
				if (t+tm)==n && !lastC[n] {
					t = n
					break
				}
				if lastC[n] {
					tm++
				}
			}

			switch (t) {
			case 0:
				z.rooms[y][x].Walls.Top = false
				y--
				z.rooms[y][x].Walls.Bottom = false
				break
			case 1:
				z.rooms[y][x].Walls.Right = false
				x++
				z.rooms[y][x].Walls.Left = false
				break
			case 2:
				z.rooms[y][x].Walls.Bottom = false
				y++
				z.rooms[y][x].Walls.Top = false
				break
			case 3:
				z.rooms[y][x].Walls.Left = false
				x--
				z.rooms[y][x].Walls.Right = false
				break
			}
			lastC = [4]bool{false, false, false, false}
			lastCell = (t+2)%4
			lastC[lastCell] = true
			stackIndex++
			stack[stackIndex] = mazelib.Coordinate{x, y}
			z.rooms[y][x].Visited = true

			c++
		}





	}


	// Random* icarus & treasure
	icarusX := rand.Intn(xSize)
	icarusY := rand.Intn(ySize)
	treasureX := rand.Intn(xSize)
	treasureY := rand.Intn(ySize)

	// *Don't let them be in the same cell, no fun then
	for ;; {
		if icarusX!=treasureX || icarusY!=treasureY {
			break
		} else {
			treasureX = rand.Intn(xSize)
			treasureY = rand.Intn(ySize)
		}
	}
	z.SetStartPoint(icarusX, icarusY)
	z.SetTreasure(treasureX, treasureY)

	return z
}

func spikyHorizontalMaze() *Maze {
	z := fullMaze()
	ySize := viper.GetInt("height")
	xSize := viper.GetInt("width")

	middleX := xSize/2
	middleY := ySize/2

	for x:=0;x<xSize;x++ {
		for y:=0;y<ySize;y++ {
			if x>0 && x!=(middleX+1) {
				z.rooms[y][x].Walls.Left = false
			}
			if x<(xSize-1) && x!=middleX {
				z.rooms[y][x].Walls.Right = false
			}
			if x==0 && y>0 {
				z.rooms[y][x].Walls.Top = false
			}
			if x==0 && y<(ySize-1) {
				z.rooms[y][x].Walls.Bottom = false
			}
			if x==(xSize-1) && y>0 {
				z.rooms[y][x].Walls.Top = false
			}
			if x==(xSize-1) && y<(ySize-1) {
				z.rooms[y][x].Walls.Bottom = false
			}
		}
	}

	z.rooms[0][middleX].Walls.Right = false
	z.rooms[ySize-1][middleX].Walls.Right = false
	z.rooms[0][middleX+1].Walls.Left = false
	z.rooms[ySize-1][middleX+1].Walls.Left = false

	z.rooms[middleY][xSize-1].Walls.Bottom = true
	z.rooms[middleY+1][xSize-1].Walls.Top = true


	// Random* icarus & treasure
	icarusX := rand.Intn(xSize)
	icarusY := rand.Intn(ySize)
	treasureX := rand.Intn(xSize)
	treasureY := rand.Intn(ySize)

	// *Don't let them be in the same cell, no fun then
	for ;; {
		if icarusX!=treasureX || icarusY!=treasureY {
			break
		} else {
			treasureX = rand.Intn(xSize)
			treasureY = rand.Intn(ySize)
		}
	}

	z.SetStartPoint(icarusX, icarusY)
	z.SetTreasure(treasureX, treasureY)

	return z
}

func spikyVerticalMaze() *Maze {
	z := fullMaze()
	ySize := viper.GetInt("height")
	xSize := viper.GetInt("width")

	middleY := ySize/2

	for x:=0;x<xSize;x++ {
		for y:=0;y<ySize;y++ {
			if y>0 && y!=(middleY) {
				z.rooms[y][x].Walls.Top = false
			}
			if y<(ySize-1) && y!=(middleY-1) {
				z.rooms[y][x].Walls.Bottom = false
			}
			if y==0 && x>0 {
				z.rooms[y][x].Walls.Left = false
			}
			if y==0 && x<(xSize-1) {
				z.rooms[y][x].Walls.Right = false
			}
			if y==(ySize-1) && x>0 {
				z.rooms[y][x].Walls.Left = false
			}
			if y==(ySize-1) && x<(xSize-1) {
				z.rooms[y][x].Walls.Right = false
			}
		}
	}

	z.rooms[middleY-1][0].Walls.Bottom = false;
	z.rooms[middleY][0].Walls.Top = false;

	// Random* icarus & treasure
	icarusX := rand.Intn(xSize)
	icarusY := rand.Intn(ySize)
	treasureX := rand.Intn(xSize)
	treasureY := rand.Intn(ySize)

	// *Don't let them be in the same cell, no fun then
	for ;; {
		if icarusX!=treasureX || icarusY!=treasureY {
			break
		} else {
			treasureX = rand.Intn(xSize)
			treasureY = rand.Intn(ySize)
		}
	}

	z.SetStartPoint(icarusX, icarusY)
	z.SetTreasure(treasureX, treasureY)

	return z
}

func voidMaze() *Maze {
	z := emptyMaze()
	ySize := viper.GetInt("height")
	xSize := viper.GetInt("width")

	for x:=0;x<xSize;x++ {
		for y:=0;y<ySize;y++ {
			if x==0 {
				z.rooms[y][x].Walls.Left = true
			}
			if x==(xSize-1) {
				z.rooms[y][x].Walls.Right = true
			}
			if y==0 {
				z.rooms[y][x].Walls.Top = true
			}
			if y==(ySize-1) {
				z.rooms[y][x].Walls.Bottom = true
			}
		}
	}


	// Random* icarus & treasure
	icarusX := rand.Intn(xSize)
	icarusY := rand.Intn(ySize)
	treasureX := rand.Intn(xSize)
	treasureY := rand.Intn(ySize)

	// *Don't let them be in the same cell, no fun then
	for ;; {
		if icarusX!=treasureX || icarusY!=treasureY {
			break
		} else {
			treasureX = rand.Intn(xSize)
			treasureY = rand.Intn(ySize)
		}
	}

	z.SetStartPoint(icarusX, icarusY)
	z.SetTreasure(treasureX, treasureY)

	return z
}

func patternMaze() *Maze {
	z := fullMaze()
	ySize := viper.GetInt("height")
	xSize := viper.GetInt("width")

	xPattern := int(math.Floor(float64(xSize/4)))
	yPattern := int(math.Floor(float64(ySize/4)))

	// Repeat human-made pattern 4x4
	for x:=0;x<xPattern;x++ {
		for y:=0;y<yPattern;y++ {
			z.rooms[4*y][4*x].Walls = mazelib.Survey{true, false, false, true}
			z.rooms[4*y][4*x+1].Walls = mazelib.Survey{true, true, true, false}
			z.rooms[4*y][4*x+2].Walls = mazelib.Survey{true, false, false, true}
			z.rooms[4*y][4*x+3].Walls = mazelib.Survey{true, true, false, false}

			z.rooms[4*y+1][4*x].Walls = mazelib.Survey{false, true, false, true}
			z.rooms[4*y+1][4*x+1].Walls = mazelib.Survey{true, false, false, true}
			z.rooms[4*y+1][4*x+2].Walls = mazelib.Survey{false, true, true, false}
			z.rooms[4*y+1][4*x+3].Walls = mazelib.Survey{false, true, true, true}

			z.rooms[4*y+2][4*x].Walls = mazelib.Survey{false, false, true, true}
			z.rooms[4*y+2][4*x+1].Walls = mazelib.Survey{false, false, false, false}
			z.rooms[4*y+2][4*x+2].Walls = mazelib.Survey{true, true, false, false}
			z.rooms[4*y+2][4*x+3].Walls = mazelib.Survey{true, true, false, true}

			z.rooms[4*y+3][4*x].Walls = mazelib.Survey{true, false, true, true}
			z.rooms[4*y+3][4*x+1].Walls = mazelib.Survey{false, true, true, false}
			z.rooms[4*y+3][4*x+2].Walls = mazelib.Survey{false, false, true, true}
			z.rooms[4*y+3][4*x+3].Walls = mazelib.Survey{false, true, true, false}

			z.rooms[4*y][4*x+3].Visited = true
			z.rooms[4*y+1][4*x+3].Visited = true
			z.rooms[4*y+2][4*x+3].Visited = true
			z.rooms[4*y+3][4*x].Visited = true
			z.rooms[4*y+3][4*x+1].Visited = true
			z.rooms[4*y+3][4*x+2].Visited = true
			z.rooms[4*y+3][4*x+3].Visited = true
		}
	}

	// Fill the non-pattern with backtrack maze
	if xSize>(xPattern*4) || ySize>(yPattern*4) {
		stackSize := ySize*xSize-(16*xPattern*yPattern)
		stackIndex := 0
		stack := make([]mazelib.Coordinate, stackSize)
		x := xSize-1
		y := ySize-1
		lastC := [4]bool{false, true, true, false}
		lastCell := 2

		stack[stackIndex] = mazelib.Coordinate{x, y}

		c := 0
		for c < stackSize{
			free := 3
			for n:=0; n<3; n++ {
				t := (1+lastCell+n)%4

				switch (t) {
				case 0:
					if (y-1)<0 {
						lastC[0] = true
						free--
					} else {
						lastC[0] = z.rooms[y-1][x].Visited
						if lastC[0] {
							free--
						}
					}
					break
				case 1:
					if (x+1)>=xSize {
						lastC[1] = true
						free--
					} else {
						lastC[1] = z.rooms[y][x+1].Visited
						if lastC[1] {
							free--
						}
					}
					break
				case 2:
					if (y+1)>=ySize {
						lastC[2] = true
						free--
					} else {
						lastC[2] = z.rooms[y+1][x].Visited
						if lastC[2] {
							free--
						}
					}
					break
				case 3:
					if (x-1)<0 {
						lastC[3] = true
						free--
					} else {
						lastC[3] = z.rooms[y][x-1].Visited
						if lastC[3] {
							free--
						}
					}
					break
				}
			}

			if free==0 {
				lastCell = (lastCell+2)%4
				lastC[lastCell] = true
				stackIndex--
				x = stack[stackIndex].X
				y = stack[stackIndex].Y
			} else {
				t := rand.Intn(free)
				tm := 0
				for n:=0; n<4; n++ {
					if (t+tm)==n && !lastC[n] {
						t = n
						break
					}
					if lastC[n] {
						tm++
					}
				}

				switch (t) {
				case 0:
					z.rooms[y][x].Walls.Top = false
					y--
					z.rooms[y][x].Walls.Bottom = false
					break
				case 1:
					z.rooms[y][x].Walls.Right = false
					x++
					z.rooms[y][x].Walls.Left = false
					break
				case 2:
					z.rooms[y][x].Walls.Bottom = false
					y++
					z.rooms[y][x].Walls.Top = false
					break
				case 3:
					z.rooms[y][x].Walls.Left = false
					x--
					z.rooms[y][x].Walls.Right = false
					break
				}
				lastC = [4]bool{false, false, false, false}
				lastCell = (t+2)%4
				lastC[lastCell] = true
				stackIndex++
				stack[stackIndex] = mazelib.Coordinate{x, y}
				z.rooms[y][x].Visited = true

				c++
			}
		}
	}

	r := 0
	for x:=0;x<xPattern;x++ {
		for y := 0; y<yPattern; y++ {
			if (4*x+3)<xSize {
				r = rand.Intn(4);
				z.rooms[4*y+r][4*x+3].Walls.Right = false
				z.rooms[4*y+r][4*x+4].Walls.Left = false
			}

			if (4*y+3)<ySize {
				r = rand.Intn(4);
				z.rooms[4*y+3][4*x+r].Walls.Bottom = false
				z.rooms[4*y+4][4*x+r].Walls.Top = false
			}
		}
	}


	// Random* icarus & treasure
	icarusX := rand.Intn(xSize)
	icarusY := rand.Intn(ySize)
	treasureX := rand.Intn(xSize)
	treasureY := rand.Intn(ySize)

	// *Don't let them be in the same cell, no fun then
	for ;; {
		if icarusX!=treasureX || icarusY!=treasureY {
			break
		} else {
			treasureX = rand.Intn(xSize)
			treasureY = rand.Intn(ySize)
		}
	}

	z.SetStartPoint(icarusX, icarusY)
	z.SetTreasure(treasureX, treasureY)

	return z
}


func createMaze() *Maze {

	// Get the maze flag to change among some types of mazes
	mazeString := viper.GetString("maze")
	if mazeString=="void" { // "empty" maze, only outer walls
		return voidMaze()
	} else if mazeString=="horizontalspiky" { // this works quite well
		return spikyHorizontalMaze()
	} else if mazeString=="verticalspiky" {
		return spikyVerticalMaze()
	} else if mazeString=="pattern" { // repeat a human-made pattern over and over
		return patternMaze()
	} else if mazeString=="backtrack" { // created using bactrack algo
		return backtrackerMaze()
	} else {
		return spikyVerticalMaze()
	}

}
