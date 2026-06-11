package models

// AccessList is a named set of IP rules used to restrict access to proxy hosts.
type AccessList struct {
	ID        int64              `json:"id"`
	Name      string             `json:"name"`
	Clients   []AccessListClient `json:"clients,omitempty"`
	CreatedAt string             `json:"created_at"`
	UpdatedAt string             `json:"updated_at"`
}

// AccessListClient is a single IP/CIDR entry in an AccessList, with a directive
// of "allow" or "deny".
type AccessListClient struct {
	ID           int64  `json:"id"`
	AccessListID int64  `json:"access_list_id"`
	Address      string `json:"address"`
	Directive    string `json:"directive"`
	SortOrder    int    `json:"sort_order"`
}
