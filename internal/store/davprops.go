package store

import "context"

// DavProp is a property a calendar client stored on a calendar.
type DavProp struct {
	Space, Local, Value string
}

// DavProps returns the client-set properties of a user's calendars, by calendar key.
func DavProps(ctx context.Context, q Q, userID string) (map[string][]DavProp, error) {
	rows, err := q.QueryContext(ctx, "SELECT calendar, space, local, value FROM dav_props WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]DavProp{}
	for rows.Next() {
		var calendar string
		var p DavProp
		if err := rows.Scan(&calendar, &p.Space, &p.Local, &p.Value); err != nil {
			return nil, err
		}
		out[calendar] = append(out[calendar], p)
	}
	return out, rows.Err()
}

func SetDavProp(ctx context.Context, q Q, userID, calendar string, p DavProp) error {
	_, err := q.ExecContext(ctx, `INSERT INTO dav_props (user_id, calendar, space, local, value) VALUES (?,?,?,?,?)
		ON CONFLICT (user_id, calendar, space, local) DO UPDATE SET value = excluded.value`,
		userID, calendar, p.Space, p.Local, p.Value)
	return err
}

func DeleteDavProp(ctx context.Context, q Q, userID, calendar, space, local string) error {
	_, err := q.ExecContext(ctx, "DELETE FROM dav_props WHERE user_id = ? AND calendar = ? AND space = ? AND local = ?",
		userID, calendar, space, local)
	return err
}
