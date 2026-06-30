import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import '../entities/gift_entity.dart';

abstract interface class GiftRepository {
  Future<Either<Failure, List<GiftEntity>>> getGifts();
  Future<Either<Failure, void>> sendGift({required String giftId, required String targetUserId, required int count});
}
