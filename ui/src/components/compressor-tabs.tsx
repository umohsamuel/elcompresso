"use client";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { CompressPanel } from "@/components/compress-panel";

export function CompressorTabs() {
  return (
    <Tabs defaultValue="video" className="w-full">
      <TabsList className="w-full grid grid-cols-3">
        <TabsTrigger value="video">Video</TabsTrigger>
        <TabsTrigger value="audio">Audio</TabsTrigger>
        <TabsTrigger value="image">Image</TabsTrigger>
      </TabsList>

      <TabsContent value="video" className="mt-6">
        <CompressPanel fileType="video" />
      </TabsContent>

      <TabsContent value="audio" className="mt-6">
        <CompressPanel fileType="audio" />
      </TabsContent>

      <TabsContent value="image" className="mt-6">
        <CompressPanel fileType="image" />
      </TabsContent>
    </Tabs>
  );
}
