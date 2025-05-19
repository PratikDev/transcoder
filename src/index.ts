Bun.serve({
	port: 3000,
	routes: {
		"/api/status": new Response("OK"),
	},
});
