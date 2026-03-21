import User, { IUser } from "../models/User";

class UserRepository {
    findByUserId(userId: string) {
        return User.findOne({ userId });
    }

    createUser(userData: Partial<IUser>) {
        return User.findOneAndUpdate(
            { userId: userData.userId },
            { $set: userData },
            { upsert: true, returnDocument: 'after' }
        );
    }

    updateUser(userId: string, updateData: Partial<IUser>) {
        return User.findOneAndUpdate({ userId }, updateData, { returnDocument: 'after' });
    }

    getAll() {
        return User.find();
    }

    deleteByUserId(userId: string) {
        return User.deleteOne({ userId });
    }

    async isAdmin(userId: string): Promise<boolean> {
        const user = await User.findOne({ userId });
        return user ? user.isAdmin : false;
    }
}

export default new UserRepository();
