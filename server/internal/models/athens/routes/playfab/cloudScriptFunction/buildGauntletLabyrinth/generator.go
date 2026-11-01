package buildGauntletLabyrinth

import (
	"fmt"
	"math"
	"math/rand/v2"
	"slices"

	"github.com/luskaner/ageServer/server/internal/models/athens/routes/playfab"
	"github.com/luskaner/ageServer/server/internal/models/athens/user"
)

import (
	mapset "github.com/deckarep/golang-set/v3"
)

const columns = 10
const rows = 6

var window = []int{-1, 0, +1}
var windowLen int

func init() {
	windowLen = len(window)
}

type Range struct {
	Min int
	Max int
}

func (r Range) RandomValue() int {
	return rand.N(r.Max-r.Min+1) + r.Min
}

var nodesPerColumn = []Range{
	// Initial column
	{Min: 2, Max: 3},
	// Middle columns
	{Min: 2, Max: 6},
	{Min: 2, Max: 6},
	{Min: 2, Max: 6},
	{Min: 2, Max: 6},
	{Min: 2, Max: 6},
	{Min: 2, Max: 6},
	{Min: 2, Max: 6},
	{Min: 2, Max: 6},
	// Boss column
	{Min: 1, Max: 1},
}

// blessingLevelPerColumn indicates the blessing level for each columm
var blessingLevelPerColumn = []int{0, 0, 1, 1, 2, 2, 3, 3, 4, 5}

func GenerateNumberOfNodes() []int {
	nodes := make([]int, columns)
	nodes[0] = nodesPerColumn[0].RandomValue()
	for col, rang := range nodesPerColumn[1 : len(nodesPerColumn)-1] {
		index := col + 1
		previousNodes := nodes[index-1]
		maxNodes := math.Min(rows, float64(previousNodes*2+1))
		minNodes := (previousNodes + windowLen - 1) / windowLen
		finalRng := Range{
			Min: int(math.Max(float64(rang.Min), float64(minNodes))),
			Max: int(math.Min(float64(rang.Max), maxNodes)),
		}
		nodes[index] = finalRng.RandomValue()
	}
	nodes[columns-1] = nodesPerColumn[columns-1].RandomValue()
	return nodes
}

func connectablePositions(position int) (positions []int) {
	for _, pos := range window {
		computedPos := position + pos
		if computedPos > -1 && computedPos < rows {
			positions = append(positions, computedPos)
		}
	}
	return
}

func computePositions(previousNodes []int, numberOfNodes int) []int {
	nodeToPositions := make(map[int]mapset.Set[int], rows)
	positionToNodes := make(map[int]mapset.Set[int], rows)
	for i := range rows {
		positionToNodes[i] = mapset.NewThreadUnsafeSet[int]()
	}
	for _, node := range previousNodes {
		positions := connectablePositions(node)
		nodeToPositions[node] = mapset.NewThreadUnsafeSet[int](positions...)
		for _, pos := range positions {
			positionToNodes[pos].Add(node)
		}
	}
	finalPositions := mapset.NewThreadUnsafeSet[int]()
	for _, positions := range nodeToPositions {
		best := -1
		bestCard := -1
		for _, pos := range positions.ToSlice() {
			card := positionToNodes[pos].Cardinality()
			if card > bestCard {
				best = pos
				bestCard = card
			}
		}
		if best != -1 {
			finalPositions.Add(best)
		}
	}
	for i := 1; i <= windowLen && finalPositions.Cardinality() < numberOfNodes; i++ {
		acumPositions := mapset.NewThreadUnsafeSet[int]()
		for _, positions := range nodeToPositions {
			if positions.Cardinality() != i {
				continue
			}
			acumPositions.AppendFrom(positions)
		}
		for j := windowLen; j > 0 && finalPositions.Cardinality() < numberOfNodes; j-- {
			var currentPositions []int
			for position, nodes := range positionToNodes {
				if nodes.Cardinality() != j || !acumPositions.Contains(position) {
					continue
				}
				if !finalPositions.Contains(position) {
					currentPositions = append(currentPositions, position)
				}
			}
			if len(currentPositions) > 1 {
				shuffle(currentPositions)
			}
			positionsToTake := min(numberOfNodes-finalPositions.Cardinality(), len(currentPositions))
			if positionsToTake > 0 {
				finalPositions.Append(currentPositions[:positionsToTake]...)
			}
		}
	}
	return finalPositions.ToSlice()
}

func shuffle[T any](slice []T) {
	rand.Shuffle(len(slice), func(i, j int) {
		slice[i], slice[j] = slice[j], slice[i]
	})
}

func GenerateNodeRows(numberOfNodes []int) [][]int {
	nodes := make([][]int, columns)
	// The first positions can be completely random
	positions := make([]int, rows)
	for i := range rows {
		positions[i] = i
	}
	shuffle(positions)
	nodes[0] = positions[:numberOfNodes[0]]
	for col, numberOfColumnNodes := range numberOfNodes[:len(numberOfNodes)-1] {
		finalCol := col + 1
		nodes[finalCol] = computePositions(nodes[finalCol-1], numberOfColumnNodes)
	}
	// Last node as it is a boss node
	nodes[len(numberOfNodes)-1] = []int{0}
	return nodes
}

func RandomizedBlessings(blessings map[int][]string) (randomBlessings map[int][]string) {
	randomBlessings = make(map[int][]string, len(blessings))
	for k, v := range blessings {
		newSlice := make([]string, len(v))
		copy(newSlice, v)
		shuffle(newSlice)
		randomBlessings[k] = newSlice
	}
	return randomBlessings
}

func GenerateMissions(nodeRows [][]int, poolsIndexes []int, missionsPools playfab.GauntletMissionPools, blessings map[int][]string) (missionColumns [][]*user.ChallengeMission) {
	missionColumns = make([][]*user.ChallengeMission, len(nodeRows))
	blessingIndexes := make(map[int]int)
	nodePosToIndex := make([]map[int]int, len(nodeRows))
	for col, nodes := range nodeRows {
		nodePosToIndex[col] = make(map[int]int, len(nodes))
		missionsColumn := make([]*user.ChallengeMission, len(nodes))
		pool := missionsPools[poolsIndexes[col]]
		allMissions := pool.Missions
		indexesChoosen := rand.Perm(len(allMissions))[:len(nodes)]
		blessingLevel := blessingLevelPerColumn[col]
		allBlessings := blessings[blessingLevel]
		for i, node := range nodes {
			var X, Y int
			var visualization string
			predecessors := make([]string, 0)
			if col == columns-1 {
				for _, predecessor := range missionColumns[col-1] {
					predecessors = append(predecessors, predecessor.Id)
				}
				X = 1600
				visualization = "UberBoss"
			} else {
				if col > 0 {
					for _, previousColumnMission := range missionColumns[col-1] {
						if !slices.Contains(connectablePositions(previousColumnMission.RowIndex), node) {
							continue
						}
						predecessors = append(predecessors, previousColumnMission.Id)
					}
				}
				Y = -350 + 140*node
				X = 80 + 160*col
				visualization = "Regular"
			}
			unusedBlessing := allBlessings[blessingIndexes[blessingLevel]]
			mission := allMissions[indexesChoosen[i]]
			missionsColumn[i] = &user.ChallengeMission{
				RowIndex:                node,
				Id:                      fmt.Sprintf("%d_%d_%s/%s", col, node, pool.Name, mission.Id),
				Predecessors:            predecessors,
				PositionX:               X,
				PositionY:               Y,
				Visualization:           visualization,
				Size:                    mission.Size,
				VictoryCondition:        "Standard",
				GameType:                "Standard",
				MapVisibility:           mission.MapVisibility,
				StartingResources:       mission.StartingResources,
				WorldTwists:             []user.WorldTwist{},
				Opponents:               []user.Opponent{},
				OpponentsFor2PlayerCoop: []user.Opponent{},
				Rewards: []user.MissionRewards{
					{
						Amount:  1,
						Scaling: "None",
						ItemId:  unusedBlessing,
					},
				},
			}
			nodePosToIndex[col][node] = i
			blessingIndexes[blessingLevel]++
		}
		missionColumns[col] = missionsColumn
	}
	// Remove 1 predecessor for each pair that cross each other (as long as there are more than 1 predecessor)
	for col := 1; col < len(missionColumns); col++ {
		currentColumn := missionColumns[col]
		currentColPosToIndex := nodePosToIndex[col]
		previousColumn := missionColumns[col-1]
		previousColPosToIndex := nodePosToIndex[col-1]
		for pos := range rows - 1 {
			var currentColumnPos *user.ChallengeMission
			if index, exists := currentColPosToIndex[pos]; !exists {
				continue
			} else {
				currentColumnPos = currentColumn[index]
			}
			var previousColumnPos *user.ChallengeMission
			if index, exists := previousColPosToIndex[pos]; !exists {
				continue
			} else {
				previousColumnPos = previousColumn[index]
			}
			nextPos := pos + 1
			var currentColumnNextPos *user.ChallengeMission
			if index, exists := currentColPosToIndex[nextPos]; !exists {
				continue
			} else {
				currentColumnNextPos = currentColumn[index]
			}
			var previousColumnNextPos *user.ChallengeMission
			if index, exists := previousColPosToIndex[nextPos]; !exists {
				continue
			} else {
				previousColumnNextPos = previousColumn[index]
			}
			nextPosPredecessorIndex := slices.Index(currentColumnPos.Predecessors, previousColumnNextPos.Id)
			if nextPosPredecessorIndex == -1 {
				continue
			}
			posPredecessorIndex := slices.Index(currentColumnNextPos.Predecessors, previousColumnPos.Id)
			if posPredecessorIndex == -1 {
				continue
			}
			if rand.N(2) == 0 {
				currentColumnPos.Predecessors = slices.Delete(currentColumnPos.Predecessors, nextPosPredecessorIndex, nextPosPredecessorIndex+1)
			} else {
				currentColumnNextPos.Predecessors = slices.Delete(currentColumnNextPos.Predecessors, posPredecessorIndex, posPredecessorIndex+1)
			}
		}
	}
	return
}
