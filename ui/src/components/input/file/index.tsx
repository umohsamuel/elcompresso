"use client";

import { useRef, useState } from "react";

interface FileInputProps {
  files: File[];
  setFiles: React.Dispatch<React.SetStateAction<File[]>>;
}

export default function FileInput({ files, setFiles }: FileInputProps) {
  const inputRef = useRef<HTMLInputElement | null>(null);

  const [isDragging, setIsDragging] = useState(false);

  const handleDragEnter = () => setIsDragging(true);
  const handleDragLeave = () => setIsDragging(false);

  function handleDrop(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault();
    const _droppedFiles = Array.from(e.dataTransfer.files);
    setFiles(_droppedFiles);
  }

  function handleDragOver(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault();
  }

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    if (e.target.files) {
      const elFiles = Array.from(e.target.files);
      setFiles(elFiles);
    }
  }

  function handleFileClick() {
    inputRef.current?.click();
  }

  return (
    <div className="max-w-lg mx-auto h-full w-full py-8 flex items-center justify-center min-h-screen">
      <div
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragEnter={handleDragEnter}
        onDragLeave={handleDragLeave}
        onClick={handleFileClick}
        className={`border-2 border-dashed border-black p-5 text-center rounded-4xl h-52 flex flex-col w-full gap-4 justify-center items-center ${
          isDragging ? "bg-green-300" : "bg-white"
        } `}
      >
        <input
          ref={inputRef}
          type="file"
          onChange={handleFileChange}
          className="hidden"
        />
        <p>Drag and drop files here</p>
        <ul className="font-medium font-mono text-xs list-disc italic">
          {files.map((file, index) => (
            <li key={index}>{file.name}</li>
          ))}
        </ul>
      </div>
    </div>
  );
}
