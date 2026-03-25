import TelegramBot from "node-telegram-bot-api";
import { BotConfig, Locals } from "../types";

export class PaymentService {
    constructor(
        private bot: TelegramBot,
        private config: BotConfig,
        private locals: Locals
    ) { }

    async sendDonationInvoice(chatId: number, amount: number) {
        const lang = this.config.lang;
        const title = this.locals[lang].donateInvoiceTitle;
        const description = this.locals[lang].donateInvoiceDesc;
        const payload = JSON.stringify({ type: "donation", amount });
        const providerToken = ""; // Empty for Telegram Stars
        const currency = "XTR";
        const prices = [{ label: "Donation", amount: amount }]; // Amount in Stars

        try {
            await this.bot.sendInvoice(
                chatId,
                title,
                description,
                payload,
                providerToken,
                currency,
                prices
            );
        } catch (err) {
            console.error("[PaymentService] Error sending invoice:", err);
        }
    }

    async handlePreCheckout(query: TelegramBot.PreCheckoutQuery) {
        // Always approve pre-checkout for donations
        await this.bot.answerPreCheckoutQuery(query.id, true);
    }

    async handleSuccessfulPayment(msg: TelegramBot.Message) {
        const lang = this.config.lang;
        const payment = msg.successful_payment;

        if (!payment) return;

        const text = this.locals[lang].donationSuccess.replace("{amount}", String(payment.total_amount));
        await this.bot.sendMessage(msg.chat.id, text);
    }
}
