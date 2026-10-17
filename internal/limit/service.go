// Package limit tracks monthly/daily usage counters and enforces transaction
// limits in the same DB transaction as the ledger posting. The schema is named
// "limits" because LIMIT is an SQL keyword. BR-25…29, 37.
package limit

// TODO: implement CheckAndConsume(ctx, customerID, category, amount) and the
// refund of consumption when a pending payment fails (erd §4.5).
