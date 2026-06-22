import Lake
open Lake DSL

package «key-rest-proofs» where
  name := "key-rest-proofs"

lean_lib «KeyRestProofs» where
  roots := #[`MaskingPipeline]
