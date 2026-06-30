import 'package:fpdart/fpdart.dart';
import 'package:tiktok_clone/core/error/exceptions.dart';
import 'package:tiktok_clone/core/error/failures.dart';
import 'package:tiktok_clone/features/gifts/domain/entities/gift_entity.dart';
import 'package:tiktok_clone/features/gifts/domain/repositories/gift_repository.dart';
import '../datasources/gift_remote_datasource.dart';

class GiftRepositoryImpl implements GiftRepository {
  const GiftRepositoryImpl(this._ds);
  final GiftRemoteDatasource _ds;

  @override
  Future<Either<Failure, List<GiftEntity>>> getGifts() async {
    try {
      return Right(await _ds.getGifts());
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode ?? 500));
    }
  }

  @override
  Future<Either<Failure, void>> sendGift({required String giftId, required String targetUserId, required int count}) async {
    try {
      await _ds.sendGift(giftId: giftId, targetUserId: targetUserId, count: count);
      return const Right(null);
    } on ServerException catch (e) {
      return Left(ServerFailure(message: e.message, statusCode: e.statusCode ?? 500));
    }
  }
}
