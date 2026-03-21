import userRepository from "../repositories/userRepository";

export class UserService {
    async ensureUser(from: { id: number; first_name: string; last_name?: string; username?: string }): Promise<void> {
        const existing = await userRepository.findByUserId(String(from.id));
        if (!existing) {
            await userRepository.createUser({
                userId: String(from.id),
                firstName: from.first_name || null,
                lastName: from.last_name || null,
                userName: from.username || null,
            });
        }
    }
}
