import axios from "axios";
import { API_BASE_URL } from "@/constants/api";

export const apiClient = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    Accept: "application/json",
  },
});
