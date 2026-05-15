"use client";

import FileInput from "@/components/input/file";
import { useState } from "react";

export default function Home() {
  const [files, setFiles] = useState<File[]>([]);

  return (
    <div className="max-w-lg mx-auto h-full w-full py-8 flex items-center justify-center min-h-screen">
      <FileInput files={files} setFiles={setFiles} />
    </div>
  );
}
