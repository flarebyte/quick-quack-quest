package designmeta

cliSpec: {
	datasets: [
		{
			id:          "sales_daily"
			format:      "csv"
			path:        "doc/design-meta/examples/input/sales.csv"
			description: "Daily store product sales transactions"
			metadata: {
				owner:       "analytics"
				primary_key: "sale_id"
			}
			fields: [
				{name: "sale_id", type: "VARCHAR", nullable: false, description: "Unique sale identifier"},
				{name: "sale_date", type: "DATE", nullable: false, description: "Date of sale event"},
				{name: "customer_id", type: "VARCHAR", nullable: false, description: "Customer foreign key"},
				{name: "product_id", type: "VARCHAR", nullable: false, description: "Product identifier"},
				{name: "quantity", type: "INTEGER", nullable: false, description: "Quantity sold"},
				{name: "amount", type: "DOUBLE", nullable: false, description: "Sale amount in local currency"},
			]
		},
		{
			id:          "customers_master"
			format:      "json"
			path:        "doc/design-meta/examples/input/customers.json"
			description: "Customer profile master records"
			metadata: {
				owner:       "crm"
				primary_key: "customer_id"
			}
			fields: [
				{name: "customer_id", type: "VARCHAR", nullable: false, description: "Unique customer identifier"},
				{name: "email", type: "VARCHAR", nullable: false, description: "Primary email address"},
				{name: "country", type: "VARCHAR", nullable: true, description: "Country code"},
				{name: "signup_date", type: "DATE", nullable: true, description: "First signup date"},
			]
		},
		{
			id:          "events_stream"
			format:      "ndjson"
			path:        "doc/design-meta/examples/input/events.ndjson"
			description: "Application behavior events for funnel analysis"
			metadata: {
				owner:       "product"
				primary_key: "event_id"
			}
			fields: [
				{name: "event_id", type: "VARCHAR", nullable: false, description: "Unique event identifier"},
				{name: "event_ts", type: "TIMESTAMP", nullable: false, description: "Event timestamp in UTC"},
				{name: "event_name", type: "VARCHAR", nullable: false, description: "Event name"},
				{name: "customer_id", type: "VARCHAR", nullable: true, description: "Associated customer id"},
				{name: "metadata_json", type: "JSON", nullable: true, description: "Arbitrary JSON payload"},
			]
		},
	]

	queries: [
		{
			id:                "sales_by_country"
			required_datasets: ["sales_daily", "customers_master"]
			parameters: [
				{name: "start_date", type: "DATE", required: true, description: "Inclusive start date"},
				{name: "end_date", type: "DATE", required: true, description: "Inclusive end date"},
			]
			sql: "SELECT c.country, SUM(s.amount) AS total_amount, COUNT(*) AS tx_count FROM sales_daily s JOIN customers_master c ON s.customer_id = c.customer_id WHERE s.sale_date BETWEEN DATE($start_date) AND DATE($end_date) GROUP BY c.country ORDER BY total_amount DESC"
		},
		{
			id:                "events_per_day"
			required_datasets: ["events_stream"]
			parameters: [
				{name: "event_name", type: "VARCHAR", required: true, description: "Event type to include"},
			]
			sql: "SELECT DATE_TRUNC('day', event_ts) AS day, COUNT(*) AS total_events FROM events_stream WHERE event_name = $event_name GROUP BY 1 ORDER BY 1"
		},
		{
			id:                "customer_360"
			required_datasets: ["customers_master", "sales_daily", "events_stream"]
			parameters: [
				{name: "customer_id", type: "VARCHAR", required: true, description: "Customer id to inspect"},
			]
			sql: "SELECT c.customer_id, c.email, COALESCE(SUM(s.amount), 0) AS lifetime_value, COUNT(DISTINCT e.event_id) AS event_count FROM customers_master c LEFT JOIN sales_daily s ON s.customer_id = c.customer_id LEFT JOIN events_stream e ON e.customer_id = c.customer_id WHERE c.customer_id = $customer_id GROUP BY c.customer_id, c.email"
		},
	]
}
