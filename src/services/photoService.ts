import TelegramBot from "node-telegram-bot-api";
import * as fs from "fs";
import * as path from "path";
import https from "https";

export class PhotoService {
    constructor(private bot: TelegramBot) {}

    async downloadPhoto(fileId: string): Promise<string | null> {
        try {
            const filePath = await this.bot.getFileLink(fileId);
            const imagesDir = path.join(__dirname, "../../public/images");
            const fileName = `${Date.now()}_${fileId}.jpg`;
            const destPath = path.join(imagesDir, fileName);

            await new Promise<void>((resolve, reject) => {
                const file = fs.createWriteStream(destPath);
                https.get(filePath, (response) => {
                    response.pipe(file);
                    file.on("finish", () => { file.close(); resolve(); });
                }).on("error", (err) => { fs.unlink(destPath, () => {}); reject(err); });
            });

            return `public/images/${fileName}`;
        } catch (err) {
            console.error("Failed to download photo:", (err as Error).message);
            return null;
        }
    }

    buildMediaGroup(photos: string[], caption: string): TelegramBot.InputMediaPhoto[] {
        return photos.map((photoPath, i) => ({
            type: "photo" as const,
            media: fs.createReadStream(path.join(__dirname, "../../", photoPath)) as any,
            ...(i === 0 ? { caption, parse_mode: "HTML" as const } : {}),
        }));
    }
}
