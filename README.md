### The Go Challenge 6

#### Author
Daniel Esteban - [@_CONEJO](https://twitter.com/_CONEJO)


#### Daedalus
Daedalus could accept the flag "maze" to generate different type of maps, accepted values are: 'prim' (default), 'circle', 'backtrack', 'verticalspiky', 'horizontalspiky', 'void', cheat & 'cheattwo'

* prim (default): Generated using Prim's algorithm, this gives the best results against my implementation of Icarus (134 avg)
* circle: Concentric circles, not so good results, but created to improve Icarus (85 avg)
* backtrack: Generated using backtracker algorithm, good, but not as good as Prim's. Probably because Icarus uses backtracker too, and has less open doors than Prim's (112 avg)
* verticalspiky (horizontalspiky): Divide the maze with vertical (horizontal) rooms, that's all. Works quite well to my surprise, better than backtrack, worse than prim (131 avg)
* void: An empty maze, only perimeter walls (78 avg)
* cheat*: Using Prim's algorithm, but since we know the position of the treasure, put it in a room with just one door (it should be a little more difficult to reach than completely random)
* cheattwo*: Just one long loop, cut it so the treasure is placed at an end. (just for fun)

\* *I consider that placing or modifying walls after Icarus and the treasure are placed but before Icarus awake is against the spirit of the challenge. Walls are **not** modified after Icarus is awake*

You could use the flag "--maze=xxx" to change the type of maze Daedalus will construct 
 
#### Icarus
Icarus IA is (mostly) based on the backtracker algorithm, two different algorithms with three different flavors each. Use the flag "ia" to change the way the next cell is choosen, accepted values are: 'samedirection' 
(default), 'random', 'mostlyright', 'classicsamedirection', 'classicrandom' & 'classicmostlyright'

We have two main algorithms, "classic" backtracker and a new, improved backtracker. The classic one is the old plain backtracker, when it can not continue forward, go back one room and check again. The new an 
improved, instead of going backwards, will find the nearest unvisited room (from the memory of Icarus) and trace a path to go there. This is really helpful if you find a "loop", instead of going all the way back, 
Icarus could continue forward and skip a lot of steps. On average, "new" algo is 1-2steps shorter than the classic backtracker. On "circle" maze, the improvement is about 23 steps shorter.

And then, you have three different flavors for each algorithm. One will keep the same direction if possible, another will choose a new direction randomly and the last one will try the options in order (top, right, 
bottom, left). I did not find significative differences using one or another decision making option.

You could use the flag "--ia=xxx" to change the type behavior Icarus will have. Use the "classic" prefix ('classicsamedirection', 'classicrandom' & 'classicmostlyright') to use the classic backtracker algorithm.
 
 
#### Warning
Icarus behaviour by default would be slow (in deciding where to go) in maps with very few walls, for example 'void', since it will calculate all possible paths to the nearest unvisited cell. It could take several 
dozens of seconds, but it will find a path eventually. In my experience, maps with few walls don't give good results, so I doubt anyone will be using one.


#### Future improvements and ideas
* Future improvements could include a dead-end flag/detector, if a path have no options, mark it as a dead-end, to avoid using it the next time we need to look for the nearest unvisited room (this doesn't happen in the 
classic version).
* An adaptive Icarus could be implemented too if it runs more than once, ie. run algo1, run algo2, ... run algoN, choose the best of them (algoX), and only run it from now. Not sure if possible, but this 
could be (probably) considered cheating, it could be applied to Daedalus too, and it could become a mouse-cat game, if Icarus do well with certain type of mazes, change them to another one.
* A graphical interface that could show where is Icarus
* Another idea is a fully automatic public online tournament/ranking. A website were people input their Daedalus/Icarus repository and the system match them against all other participants 



