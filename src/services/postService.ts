import TelegramBot from "node-telegram-bot-api";
import { BotConfig, Locals } from "../types";
import { PhotoService } from "./photoService";

export interface PostData {
    title: string;
    description: string;
    price: number;
    location: string;
    photos: string[];
    userId: number;
    username?: string;
    firstName: string;
}

export class PostService {
    constructor(
        private bot: TelegramBot,
        private config: BotConfig,
        private locals: Locals,
        private photoService: PhotoService
    ) {}

    private get lang() {
        return this.config.lang;
    }

    formatUserMention(userId: number, username?: string, firstName?: string): string {
        return username
            ? `@${username}`
            : `<a href="tg://user?id=${userId}">${firstName || "User"}</a>`;
    }

    formatPostText(data: PostData): string {
        return [
            `<b>${data.title}</b>`,
            data.description,
            `💰 ${data.price}`,
            `📍 ${data.location}`,
            `👤 ${this.formatUserMention(data.userId, data.username, data.firstName)}`,
        ].join("\n");
    }

    async sendPreview(chatId: number, text: string, photos: string[]): Promise<void> {
        const previewText = `${this.locals[this.lang].preview}\n${text}`;

        if (photos.length > 0) {
            const media = this.photoService.buildMediaGroup(photos, previewText);
            await this.bot.sendMediaGroup(chatId, media);
        } else {
            await this.bot.sendMessage(chatId, previewText, { parse_mode: "HTML" });
        }
    }

    async sendToModeration(postId: string, text: string, photos: string[]): Promise<void> {
        const moderationGroupId = this.config.moderationGroupId;
        const moderationTopicId = this.config.moderationTopicId;

        const approveRejectMarkup = {
            reply_markup: {
                inline_keyboard: [[
                    { text: this.locals[this.lang].approveButton, callback_data: `approve_${postId}` },
                    { text: this.locals[this.lang].rejectButton, callback_data: `reject_${postId}` },
                ]],
            },
        };

        if (photos.length > 0) {
            const media = this.photoService.buildMediaGroup(photos, text);

            await this.bot.sendMediaGroup(moderationGroupId, media, {
                reply_to_message_id: moderationTopicId,
            } as any);

            await this.bot.sendMessage(moderationGroupId, this.locals[this.lang].moderationPrompt, {
                reply_to_message_id: moderationTopicId,
                ...approveRejectMarkup,
            } as any);
        } else {
            await this.bot.sendMessage(moderationGroupId, text, {
                parse_mode: "HTML",
                reply_to_message_id: moderationTopicId,
                ...approveRejectMarkup,
            } as any);
        }
    }

    async sendToApproved(text: string, photos: string[]): Promise<void> {
        const approvedGroupId = this.config.approvedGroupId;
        const approvedTopicId = this.config.approvedTopicId;

        if (photos.length > 0) {
            const media = this.photoService.buildMediaGroup(photos, text);

            await this.bot.sendMediaGroup(approvedGroupId, media, {
                reply_to_message_id: approvedTopicId,
            } as any);
        } else {
            await this.bot.sendMessage(approvedGroupId, text, {
                parse_mode: "HTML",
                reply_to_message_id: approvedTopicId,
            } as any);
        }
    }
}
