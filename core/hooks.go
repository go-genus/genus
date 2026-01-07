package core

// AfterCreater é um hook executado após criar um registro.
// É chamado depois do INSERT no banco de dados e após o ID ser atribuído ao model.
type AfterCreater interface {
	AfterCreate() error
}

// BeforeUpdater é um hook executado antes de atualizar um registro.
// É chamado antes do UPDATE no banco de dados.
// Pode ser usado para validação ou modificação de dados antes da atualização.
type BeforeUpdater interface {
	BeforeUpdate() error
}

// AfterUpdater é um hook executado após atualizar um registro.
// É chamado depois do UPDATE no banco de dados.
type AfterUpdater interface {
	AfterUpdate() error
}

// BeforeDeleter é um hook executado antes de deletar um registro.
// É chamado antes do DELETE no banco de dados (ou soft delete).
// Pode ser usado para validação ou limpeza de recursos relacionados.
type BeforeDeleter interface {
	BeforeDelete() error
}

// AfterDeleter é um hook executado após deletar um registro.
// É chamado depois do DELETE no banco de dados (ou soft delete).
type AfterDeleter interface {
	AfterDelete() error
}

// BeforeSaver é um hook executado antes de qualquer operação de save (Create ou Update).
// É chamado antes de BeforeCreate ou BeforeUpdate.
// Útil para validações ou modificações que se aplicam tanto a criação quanto atualização.
type BeforeSaver interface {
	BeforeSave() error
}

// AfterSaver é um hook executado após qualquer operação de save (Create ou Update).
// É chamado depois de AfterCreate ou AfterUpdate.
type AfterSaver interface {
	AfterSave() error
}
