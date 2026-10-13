// Build-time flag shared by the browser and the server proxy. Never enable by URL.
export const demoMode = process.env.NEXT_PUBLIC_DEMO_MODE === "true";
