import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import '../repositories/gift_repository.dart';

class SendGiftUsecase {
  const SendGiftUsecase(this._repo);
  final GiftRepository _repo;
  Future<Either<Failure, void>> call({required String giftId, required String targetUserId, required int count}) =>
      _repo.sendGift(giftId: giftId, targetUserId: targetUserId, count: count);
}
