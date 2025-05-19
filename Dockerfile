# Use official Bun base image
FROM oven/bun:1.2.4

# Set working directory
WORKDIR /app

# Install ffmpeg and clean up
RUN apt-get update && \
  apt-get install -y ffmpeg && \
  apt-get clean && \
  rm -rf /var/lib/apt/lists/*

# Copy package files
COPY bun.lock package.json ./

# Install dependencies
RUN bun install --frozen-lockfile

# Copy the rest of the project files
COPY . .

# Build the Bun app
RUN bun build src/index.ts --compile --outfile transcoder

# Expose the port your Bun app runs on (change if needed)
EXPOSE 3000

# Run your Bun app
CMD ["./transcoder"]
