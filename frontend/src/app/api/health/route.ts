export function GET() {
  return Response.json({
    code: 0,
    message: "success",
    data: {
      status: "ok",
      service: "NexaFlow Web",
      checked_at: new Date().toISOString(),
    },
  });
}
