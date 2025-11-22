package database

// This file contains examples of proper query timeout usage.
// DO NOT use this file in production - it's for documentation only.

/*
QUERY TIMEOUT BEST PRACTICES

1. Simple Queries (2s timeout) - Use WithSimpleQueryTimeout()
   - Single row lookups by ID
   - INSERT/UPDATE of single records
   - Simple SELECT with WHERE id = $1

Example:
	ctx, cancel := dbAdapter.WithSimpleQueryTimeout(ctx)
	defer cancel()
	merchant, err := queries.GetMerchantByID(ctx, merchantID)

2. Complex Queries (5s timeout) - Use WithComplexQueryTimeout()
   - JOINs across multiple tables
   - WHERE clauses with multiple conditions
   - Aggregations (COUNT, SUM, AVG)
   - Pagination with ORDER BY

Example:
	ctx, cancel := dbAdapter.WithComplexQueryTimeout(ctx)
	defer cancel()
	transactions, err := queries.ListTransactionsByMerchant(ctx, sqlc.ListTransactionsByMerchantParams{
		MerchantID: merchantID,
		Limit:      100,
		Offset:     0,
	})

3. Report Queries (30s timeout) - Use WithReportQueryTimeout()
   - Analytics queries
   - Historical data aggregations
   - Large result sets
   - Complex reporting

Example:
	ctx, cancel := dbAdapter.WithReportQueryTimeout(ctx)
	defer cancel()
	stats, err := queries.GetMonthlyPaymentStats(ctx, sqlc.GetMonthlyPaymentStatsParams{
		StartDate: startDate,
		EndDate:   endDate,
	})

SAFETY NET:
All queries have a default 5-second timeout at the PostgreSQL connection level.
This prevents runaway queries even if application code forgets to set a timeout.

IMPORTANT:
Always use defer cancel() immediately after creating the timeout context to
prevent context leaks.
*/
