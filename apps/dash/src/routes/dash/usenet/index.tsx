import { createFileRoute } from "@tanstack/react-router";
import { useLocalStorage } from "react-use";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

import { IndexersTab } from "./-indexers-tab";
import { ServersTab } from "./-servers-tab";

export const Route = createFileRoute("/dash/usenet/")({
  component: RouteComponent,
  staticData: {
    crumb: "Stats",
  },
});

function RouteComponent() {
  const [tab = "servers", setTab] = useLocalStorage(
    "dash/usenet/stats:selected-tab",
    "servers",
  );

  return (
    <Tabs className="flex flex-col gap-4" onValueChange={setTab} value={tab}>
      <TabsList>
        <TabsTrigger value="servers">Servers</TabsTrigger>
        <TabsTrigger value="indexers">Indexers</TabsTrigger>
      </TabsList>
      <TabsContent value="servers">
        <ServersTab />
      </TabsContent>
      <TabsContent value="indexers">
        <IndexersTab />
      </TabsContent>
    </Tabs>
  );
}
