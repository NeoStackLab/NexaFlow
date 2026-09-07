import { redirect } from "next/navigation";
import { getServerInstallStatus } from "@/lib/install/server";

export const dynamic = "force-dynamic";

export default async function Home() {
  const installation = await getServerInstallStatus();
  redirect(installation?.installed ? "/login" : "/install");
}
