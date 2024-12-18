// Code generated by "stringer -type=Role"; DO NOT EDIT.

package node

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Follower-0]
	_ = x[Candidate-1]
	_ = x[Leader-2]
}

const _Role_name = "FollowerCandidateLeader"

var _Role_index = [...]uint8{0, 8, 17, 23}

func (i Role) String() string {
	if i < 0 || i >= Role(len(_Role_index)-1) {
		return "Role(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Role_name[_Role_index[i]:_Role_index[i+1]]
}
