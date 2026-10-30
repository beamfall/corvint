import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["unit.test.js"],
    reporters: ["default"],
  },
});
