import "dotenv/config";
import TelegramBot from "node-telegram-bot-api";
import { connectDB } from "./config/db";
import { BotController } from "./controllers/botController";

const token = process.env.BOT_TOKEN;
if (!token) {
    console.error("BOT_TOKEN is missing. Set it in your .env file.");
    process.exit(1);
}

connectDB();

const bot = new TelegramBot(token, { polling: true });

const controller = new BotController(bot);
controller.registerRoutes();

console.log("Bot is running...");

// /help command
bot.onText(/\/help/, (msg) => {
    const helpText = [
        "/start  - Start the bot",
        "/help   - Show this help message",
    ].join("\n");

    bot.sendMessage(msg.chat.id, helpText);
});

// Handle unknown messages (only when user is not in a conversation flow)
// Removed: the catch-all handler was stealing replies meant for input() listeners

// Graceful shutdown
process.on("SIGINT", () => {
    console.log("Shutting down bot...");
    bot.stopPolling();
    process.exit(0);
});
