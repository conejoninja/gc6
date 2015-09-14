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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/golangchallenge/gc6/mazelib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"math"
	"math/rand"
	"os"
)

type VirtualMaze struct {
	Coords 		mazelib.Coordinate
	Walls		mazelib.Survey
	Visited		bool
}

// Defining the icarus command.
// This will be called as 'laybrinth icarus'
var icarusCmd = &cobra.Command{
	Use:     "icarus",
	Aliases: []string{"client"},
	Short:   "Start the laybrinth solver",
	Long: `Icarus wakes up to find himself in the middle of a Labyrinth.
  Due to the darkness of the Labyrinth he can only see his immediate cell and if
  there is a wall or not to the top, right, bottom and left. He takes one step
  and then can discover if his new cell has walls on each of the four sides.

  Icarus can connect to a Daedalus and solve many laybrinths at a time.`,
	Run: func(cmd *cobra.Command, args []string) {
		RunIcarus()
	},
}

func init() {
	RootCmd.AddCommand(icarusCmd)
}

func RunIcarus() {
	// Run the solver as many times as the user desires.
	fmt.Println("Solving", viper.GetInt("times"), "times")
	for x := 0; x < viper.GetInt("times"); x++ {

		solveMaze()
	}

	// Once we have solved the maze the required times, tell daedalus we are done
	makeRequest("http://127.0.0.1:" + viper.GetString("port") + "/done")
}

// Make a call to the laybrinth server (daedalus) that icarus is ready to wake up
func awake() mazelib.Survey {
	contents, err := makeRequest("http://127.0.0.1:" + viper.GetString("port") + "/awake")
	if err != nil {
		fmt.Println(err)
	}
	r := ToReply(contents)
	return r.Survey
}

// Make a call to the laybrinth server (daedalus)
// to move Icarus a given direction
// Will be used heavily by solveMaze
func Move(direction string) (mazelib.Survey, error) {
	if direction == "left" || direction == "right" || direction == "up" || direction == "down" {

		contents, err := makeRequest("http://127.0.0.1:" + viper.GetString("port") + "/move/" + direction)
		if err != nil {
			return mazelib.Survey{}, err
		}

		rep := ToReply(contents)
		if rep.Victory == true {
			fmt.Println(rep.Message)
			// os.Exit(1)
			return rep.Survey, mazelib.ErrVictory
		} else {
			return rep.Survey, errors.New(rep.Message)
		}
	}

	return mazelib.Survey{}, errors.New("invalid direction")
}

// utility function to wrap making requests to the daedalus server
func makeRequest(url string) ([]byte, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	return contents, nil
}

// Handling a JSON response and unmarshalling it into a reply struct
func ToReply(in []byte) mazelib.Reply {
	res := &mazelib.Reply{}
	json.Unmarshal(in, &res)
	return *res
}


/**
 * Icarus will create a virtual map of the maze to keep track of the visited cells (visited)
 * Will also have a list the current path taken from the starting point (path)
 *
 */
func backtrackerClassicIcarus() {
	// Assume the size of the maze is unknown, even if for this challenge is fixed
	mapSize := 200
	pathIndex :=0
	// Grow a 1D array is easier than 2D array
	visited := make([]bool, mapSize)
	path := make([]mazelib.Coordinate, viper.GetInt("max-steps"))
	previousDirection := rand.Intn(4)
	// Add 1 so it doesn't complain of unused variable (depends on the IA choosen it might not be used)
	previousDirection++
	z := coordsToInt(0, 0)


	x := 0
	y := 0
	walls := awake();
	err := errors.New("none")
	visited[z] = true
	path[z] = mazelib.Coordinate{0, 0}
	for r:=0;r<viper.GetInt("max-steps");r++ { // It's a good idea to limit the step Icarus could take, so it doesn't walk forever, but it's already limited by Daedalus
		goBack := true

		//previous direction (default option)
		nr := previousDirection
		if viper.GetString("ia")=="classicrandom" {
			//random decision making
			nr = rand.Intn(4)
		} else if viper.GetString("ia")=="classicmostlyright"{
			// mostly right turns
			nr = 0
		}

		for w:=0;w<4;w++ {

			n := (nr+w)%4

			if (n==0 && !walls.Top) || (n==1 && !walls.Right) || (n==2 && !walls.Bottom) || (n==3 && !walls.Left) {
				nx := x
				ny := y
				switch(n) {
				case 0:
					ny = y-1
					z = coordsToInt(x, y-1) // maze is a 1D array, so we need a function f(x,y) = z where z is unique foreach x,y pair
					break
				case 1:
					nx = x+1
					z = coordsToInt(x+1, y)
					break
				case 2:
					ny = y+1
					z = coordsToInt(x, y+1)
					break
				case 3:
					nx = x-1
					z = coordsToInt(x-1, y)
					break
				}
				// we may want to extend our virtual maze
				for ; z>=mapSize; {
					visited, mapSize = extendVisited(visited, mapSize)
				}

				if !visited[z] {
					visited[z] = true
					walls, err = moveTo(n)
					goBack = false
					if err==mazelib.ErrVictory {
						r = viper.GetInt("max-steps")+1 //break the outer loop (steps)
						break
					}
					previousDirection = n
					x = nx
					y = ny
					pathIndex++
					path[pathIndex] = mazelib.Coordinate{x, y}

					break
				}
			}
		}
		if goBack {

			// FIND NEAREST non visited cell?


			pathIndex--
			if pathIndex<0 {
				// This should never happens, it means we have to go back further than the starting cell
				fmt.Println("No path to the treasure")
				os.Exit(3)
			}
			coords := path[pathIndex]
			if coords.Y<y {
				walls, _ = moveTo(0)
			} else if coords.X>x {
				walls, _ = moveTo(1)
			} else if coords.Y>y {
				walls, _ = moveTo(2)
			} else  {
				walls, _ = moveTo(3)
			}
			x = coords.X
			y = coords.Y
		}
	}

}

/**
 * Icarus will create a virtual map of the maze to keep track of the visited cells (visited)
 * Will also have a list the current path taken from the starting point (path)
 *
 */
func backtrackerIcarus() {
	// Assume the size of the maze is unknown, even if for this challenge is fixed
	mapSize := 200
	pathIndex :=0
	// Grow a 1D array is easier than 2D array
	virtual := make([]VirtualMaze, mapSize)
	path := make([]mazelib.Coordinate, viper.GetInt("max-steps"))
	previousDirection := rand.Intn(4)
	// Add 1 so it doesn't complain of unused variable (depends on the IA choosen it might not be used)
	previousDirection++
	z := coordsToInt(0, 0)


	x := 0
	y := 0
	walls := awake();
	err := errors.New("none")
	virtual[z].Visited = true
	virtual[z].Walls = walls
	path[z] = mazelib.Coordinate{0, 0}
	for r:=0;r<viper.GetInt("max-steps");r++ { // It's a good idea to limit the step Icarus could take, so it doesn't walk forever, but it's already limited by Daedalus
		goBack := true

		//previous direction (default option)
		nr := previousDirection
		if viper.GetString("ia")=="random" {
			//random decision making
			nr = rand.Intn(4)
		} else if viper.GetString("ia")=="mostlyright" {
			// mostly right turns
			nr = 0
		}

		for w:=0;w<4;w++ {

			n := (nr+w)%4

			if (n==0 && !walls.Top) || (n==1 && !walls.Right) || (n==2 && !walls.Bottom) || (n==3 && !walls.Left) {
				nx := x
				ny := y
				switch(n) {
				case 0:
					ny = y-1
					z = coordsToInt(x, y-1) // maze is a 1D array, so we need a function f(x,y) = z where z is unique foreach x,y pair
					break
				case 1:
					nx = x+1
					z = coordsToInt(x+1, y)
					break
				case 2:
					ny = y+1
					z = coordsToInt(x, y+1)
					break
				case 3:
					nx = x-1
					z = coordsToInt(x-1, y)
					break
				}
				// we may want to extend our virtual maze
				for ; z>=mapSize; {
					virtual, mapSize = extendVirtual(virtual, mapSize)
				}

				if !virtual[z].Visited {
					virtual[z].Visited = true
					walls, err = moveTo(n)
					virtual[z].Walls = walls
					virtual[z].Coords = mazelib.Coordinate{nx, ny}
					goBack = false
					if err==mazelib.ErrVictory {
						r = viper.GetInt("max-steps")+1 //break the outer loop (steps)
						break
					}
					previousDirection = n
					x = nx
					y = ny
					pathIndex++
					path[pathIndex] = mazelib.Coordinate{x, y}

					break
				}
			}
		}
		if goBack {

			// FIND NEAREST non visited cell?
			nPath := make([]mazelib.Coordinate, 1, viper.GetInt("max-steps"))
			nPath[0] = mazelib.Coordinate{x, y}
			newPath, newLength := nearestUnvisited(virtual, nPath, viper.GetInt("max-steps"))

			if newLength==viper.GetInt("max-steps") || newLength<2 {
				// ERROR
				newPath = make([]mazelib.Coordinate, 2, viper.GetInt("max-steps"))
				newPath[1] = mazelib.Coordinate{path[pathIndex-1].X, path[pathIndex-1].Y}
				newLength = 2
			}

			for p:=1;p<newLength;p++ {
				if newPath[p].X<x {
					walls, err = moveTo(3)
					previousDirection = 3
				} else if newPath[p].X>x {
					walls, err = moveTo(1)
					previousDirection = 1
				} else if newPath[p].Y<y {
					walls, err = moveTo(0)
					previousDirection = 0
				} else if newPath[p].Y>y {
					walls, err = moveTo(2)
					previousDirection = 2
				}
				x = newPath[p].X
				y = newPath[p].Y
				z = coordsToInt(x, y)
				for ; z>=mapSize; {
					virtual, mapSize = extendVirtual(virtual, mapSize)
				}
				virtual[z].Visited = true
				virtual[z].Walls = walls

				if err==mazelib.ErrVictory {
					r = viper.GetInt("max-steps")+1 //break the outer loop (steps)
					break
				}

			}

		}
	}

}

func nearestUnvisited(maze []VirtualMaze, path []mazelib.Coordinate, shortestLen int) ([]mazelib.Coordinate, int) {
	l := len(maze)
	lp := len(path)
	if lp>=shortestLen {
		return make([]mazelib.Coordinate,1), shortestLen
	}
	coords := path[lp-1]
	z := coordsToInt(coords.X, coords.Y)
	var tmpPath []mazelib.Coordinate
	tmpLen := viper.GetInt("max-steps")
	var shortestPath []mazelib.Coordinate
	if z<l {
		walls := maze[z].Walls

		for i:=0;i<4;i++ {
			tmpX := coords.X-1
			tmpY := coords.Y
			tmpWall := walls.Left
			if i==0 {
				tmpX = coords.X
				tmpY = coords.Y-1
				tmpWall = walls.Top
			} else if i==1 {
				tmpX = coords.X+1
				tmpY = coords.Y
				tmpWall = walls.Right
			} else if i==2 {
				tmpX = coords.X
				tmpY = coords.Y+1
				tmpWall = walls.Bottom
			}
			if !tmpWall && (lp==1 || (lp>1 && (tmpX!=path[lp-2].X || tmpY!=path[lp-2].Y))) {

				found := false
				for p:=(lp-1);p>=0;p-- {
					if path[p].X==tmpX && path[p].Y==tmpY {
						found = true
						break
					}
				}

				if !found {
					z = coordsToInt(tmpX, tmpY)
					tmpPath = path[0 : lp+1]
					tmpPath[lp] = mazelib.Coordinate{tmpX, tmpY}
					//fmt.Println("ADD CELL", tmpX, tmpY, maze[z].Walls, walls, tmpPath)
					tmpLen = lp+1

					if z<l && maze[z].Visited && tmpLen<shortestLen {
						tmpPath, tmpLen = nearestUnvisited(maze, tmpPath, shortestLen)
					}

					if tmpLen<shortestLen {
						shortestLen = tmpLen
						shortestPath = make([]mazelib.Coordinate, tmpLen) //tmpPath[0:tmpLen]
						copy(shortestPath, tmpPath)
					}
				}
			}
		}

	} else {
		return path, lp
	}

	return shortestPath, shortestLen
}


// little wrapper as it's easier to work with int than strings for the directions
func moveTo(n int) (mazelib.Survey, error) {
	if n==0 {
		return Move("up")
	} else if n==1 {
		return Move("right")
	} else if n==2 {
		return Move("down")
	}
	return Move("left")
}

/**
 * f(x,y) = z, foreach x,y pair, exists an unique z (cantor pairing)
 * We start in coords 0,0 (this could be in the middle of the maze)
 * so we could have positive and negative coords
 *
 * Note: This could be a problem with some big mazes, as the numbers grow quite fast, and we may be interested in using BigNumbers
 */
func coordsToInt(x, y int) int {
	z := 0
	for k:=1;k<=int(math.Abs(float64(x))+math.Abs(float64(y)));k++ {
		z += k
	}
	z += int(math.Abs(float64(y)))
	z *= 4
	if x>=0 && y<0 {
		z += 1
	} else if x<0 && y>=0 {
		z += 2
	} else if x>=0 && y>=0 {
		z += 3
	}
	return z
}

func extendVisited(labyrinth []bool, size int) ([]bool, int) {
	newSize := size+200
	newLabyrinth := make([]bool, newSize)
	copy(newLabyrinth, labyrinth)
	return newLabyrinth, newSize
}

func extendVirtual(labyrinth []VirtualMaze, size int) ([]VirtualMaze, int) {
	newSize := size+200
	newLabyrinth := make([]VirtualMaze, newSize)
	copy(newLabyrinth, labyrinth)
	return newLabyrinth, newSize
}

func solveMaze() {
	if viper.GetString("ia")=="classicrandom" || viper.GetString("ia")=="classicmostlyright" || viper.GetString("ia")=="classicsamedirection" {
		backtrackerClassicIcarus()
	} else {
		backtrackerIcarus()
	}
}
