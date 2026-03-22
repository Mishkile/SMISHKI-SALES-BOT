import mongoose from "mongoose";

// Default to localhost for safety, Docker should always provide env variable
const MONGO_URI = process.env.MONGO_URI || "mongodb://localhost:27017/SalesBotDB";

export async function connectDB(): Promise<void> {
    try {
        await mongoose.connect(MONGO_URI);
        console.log("✅ Connected to MongoDB");
    } catch (err) {
        console.error("❌ MongoDB connection error:", (err as Error).message);

        // Instead of process.exit(1), we wait and retry
        console.log("Retrying connection in 5 seconds...");
        setTimeout(connectDB, 5000);
    }
}

// Log connection issues after the initial connection
mongoose.connection.on('error', (err) => {
    console.error("⚠️ MongoDB runtime error:", err);
});

mongoose.connection.on('disconnected', () => {
    console.warn("⚠️ MongoDB disconnected. Attempting to reconnect...");
});