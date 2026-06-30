import 'package:fpdart/fpdart.dart';

import '../../../../core/error/failures.dart';
import '../../../../core/usecases/usecase.dart';
import '../entities/order_entity.dart';
import '../repositories/ecommerce_repository.dart';

class AddToCartParams {
  const AddToCartParams({
    required this.productId,
    this.variantId,
    this.qty = 1,
  });
  final String productId;
  final String? variantId;
  final int qty;
}

class AddToCartUseCase implements UseCase<CartEntity, AddToCartParams> {
  const AddToCartUseCase(this._repository);
  final EcommerceRepository _repository;

  @override
  Future<Either<Failure, CartEntity>> call(AddToCartParams params) async {
    return _repository.addToCart(
      productId: params.productId,
      variantId: params.variantId,
      qty: params.qty,
    );
  }
}

class UpdateCartItemParams {
  const UpdateCartItemParams({required this.itemId, required this.qty});
  final String itemId;
  final int qty;
}

class UpdateCartItemUseCase
    implements UseCase<CartEntity, UpdateCartItemParams> {
  const UpdateCartItemUseCase(this._repository);
  final EcommerceRepository _repository;

  @override
  Future<Either<Failure, CartEntity>> call(UpdateCartItemParams params) async {
    return _repository.updateCartItem(
        itemId: params.itemId, qty: params.qty);
  }
}

class RemoveCartItemUseCase {
  const RemoveCartItemUseCase(this._repository);
  final EcommerceRepository _repository;

  Future<Either<Failure, CartEntity>> call(String itemId) async {
    return _repository.removeCartItem(itemId);
  }
}

class GetCartUseCase implements UseCase<CartEntity, NoParams> {
  const GetCartUseCase(this._repository);
  final EcommerceRepository _repository;

  @override
  Future<Either<Failure, CartEntity>> call(NoParams params) async {
    return _repository.getCart();
  }
}
