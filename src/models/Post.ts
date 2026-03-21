import mongoose, { Schema, Document } from "mongoose";

export interface IPost extends Document {
    userId: string;
    status: "pending" | "approved" | "rejected" | "sold";
    price: number;
    title: string;
    description: string;
    location: string;
    photos: string[];
    createdAt: Date;
    isExpired: boolean;
}

const postSchema = new Schema<IPost>({
    userId: { type: String, required: true },
    status: { type: String, enum: ["pending", "approved", "rejected", "sold"], default: "pending" },
    price: { type: Number, required: true },
    title: { type: String, required: true },
    description: { type: String, default: "" },
    location: { type: String, default: "" },
    photos: [{ type: String }],
    createdAt: { type: Date, default: Date.now },
    isExpired: { type: Boolean, default: false },
});

export default mongoose.model<IPost>("Post", postSchema);
