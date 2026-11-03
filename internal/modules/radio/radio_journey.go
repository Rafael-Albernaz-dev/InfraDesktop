package radio

import (
	"sort"
	"strconv"
)

func radioIndexKey(q Command) string { return q.Scope.Profile + "|" + q.Scope.Activation }
func radioIndexContext(st State) string {
	out := ""
	for _, d := range st.Devices {
		if d.Active && d.Role == RoleAP {
			out += d.ID + "|" + PPConnection(d) + "|" + strconv.FormatUint(d.Revision, 10)
			if c := credential(&st, d.CredentialID); c != nil {
				out += "|" + strconv.FormatUint(c.Metadata.Revision, 10)
			}
		}
	}
	return out
}

// Bound public metadata as well as hidden registration entries. A large inventory
// must still expose refused APs without exceeding the IPC frame budget.
func pageRadioIndex(v RadioView, offset int) (RadioView, Code) {
	if v.Index == nil {
		return v, OK
	}
	keys := make([]string, 0, len(v.Index.Targets))
	for id := range v.Index.Targets {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	if offset < 0 || offset > len(keys) {
		return RadioView{}, CursorInvalid
	}
	index := *v.Index
	index.Targets = map[string]Code{}
	index.Labels = map[string]string{}
	end := min(offset+RadioPageSize, len(keys))
	index.Total, index.Offset, index.Next, index.More = len(keys), offset, end, end < len(keys)
	for _, id := range keys[offset:end] {
		index.Targets[id] = v.Index.Targets[id]
		index.Labels[id] = v.Index.Labels[id]
	}
	v.Index = &index
	return v, OK
}
